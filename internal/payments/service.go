package payments

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	cfg             Config
	store           *OrderStore
	controlPlane    *ControlPlaneClient
	logger          *slog.Logger
	httpServer      *http.Server
	orderTpl        *template.Template
	landingTpl      *template.Template
	loginTpl        *template.Template
	moderatorTpl    *template.Template
	moderatorCookie string
	plansByID       map[string]PlanConfig
}

func New(cfg Config) *Service {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store := NewOrderStore(cfg.StoragePath)
	service := &Service{
		cfg:             cfg,
		store:           store,
		controlPlane:    NewControlPlaneClient(cfg.ControlPlaneBaseURL, cfg.ControlPlaneToken),
		logger:          logger.With("component", "pay-service"),
		moderatorCookie: "novpn_pay_admin_token",
		plansByID:       make(map[string]PlanConfig, len(cfg.Plans)),
	}
	for _, plan := range cfg.Plans {
		service.plansByID[strings.TrimSpace(plan.ID)] = plan
	}

	funcs := template.FuncMap{
		"formatMoney": func(minor int64, currency string) string {
			return formatMoney(minor, currency)
		},
		"statusLabel": statusLabel,
		"statusClass": func(status OrderStatus) string {
			return string(status)
		},
		"formatTime": func(value time.Time) string {
			return value.Local().Format("02.01.2006 15:04")
		},
		"formatTimePtr": func(value *time.Time) string {
			if value == nil {
				return ""
			}
			return value.Local().Format("02.01.2006 15:04")
		},
		"joinLines": func(values []string) string {
			if len(values) == 0 {
				return ""
			}
			return strings.Join(values, "\n")
		},
	}
	service.landingTpl = template.Must(template.New("landing").Funcs(funcs).Parse(landingTemplate))
	service.orderTpl = template.Must(template.New("order").Funcs(funcs).Parse(orderTemplate))
	service.loginTpl = template.Must(template.New("login").Funcs(funcs).Parse(moderatorLoginTemplate))
	service.moderatorTpl = template.Must(template.New("moderator").Funcs(funcs).Parse(moderatorOrdersTemplate))
	service.httpServer = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: service.routes(),
	}
	return service
}

func (s *Service) Start() error {
	listener, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("serve http", "error", serveErr)
		}
	}()
	s.logger.Info("pay-service listening", "addr", s.cfg.ListenAddr)
	return nil
}

func (s *Service) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(s.downloadsDir()))))
	mux.HandleFunc("/order/", s.handleOrderRoutes)
	mux.HandleFunc("/order", s.handleCreateOrder)
	mux.HandleFunc("/moderator/login", s.handleModeratorLogin)
	mux.HandleFunc("/moderator/logout", s.handleModeratorLogout)
	mux.HandleFunc("/moderator/orders/", s.handleModeratorOrderRoutes)
	mux.HandleFunc("/moderator", s.redirectModeratorOrders)
	mux.HandleFunc("/moderator/orders", s.handleModeratorOrders)
	mux.HandleFunc("/", s.handleLanding)
	return s.withLogging(mux)
}

func (s *Service) downloadsDir() string {
	return filepath.Join(filepath.Dir(s.cfg.StoragePath), "downloads")
}

func (s *Service) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func (s *Service) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.landingTpl.Execute(w, map[string]any{
		"BrandName":          s.cfg.BrandName,
		"Plans":              s.cfg.Plans,
		"PaymentCardNumber":  s.cfg.PaymentCardNumber,
		"PaymentCardHolder":  s.cfg.PaymentCardHolder,
		"PaymentCardBank":    s.cfg.PaymentCardBank,
		"AndroidLauncherURL": s.cfg.AndroidLauncherURL,
		"WindowsLauncherURL": s.cfg.WindowsLauncherURL,
		"HappDownloadURL":    s.cfg.HappDownloadURL,
		"SupportLink":        s.cfg.SupportLink,
	})
}

func (s *Service) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(12 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	plan, ok := s.plansByID[strings.TrimSpace(r.FormValue("plan_id"))]
	if !ok {
		http.Error(w, "plan not found", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("payment_screenshot")
	if err != nil {
		http.Error(w, "payment screenshot is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	orderID, err := newOrderID()
	if err != nil {
		http.Error(w, "failed to create order id", http.StatusInternalServerError)
		return
	}
	accessToken, err := newAccessToken()
	if err != nil {
		http.Error(w, "failed to create access token", http.StatusInternalServerError)
		return
	}

	order := Order{
		ID:                     orderID,
		AccessToken:            accessToken,
		Status:                 OrderStatusPendingReview,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
		PlanID:                 plan.ID,
		PlanName:               plan.Name,
		PlanBadge:              plan.Badge,
		PlanDescription:        plan.Description,
		PriceMinor:             plan.PriceMinor,
		Currency:               plan.Currency,
		DurationDays:           plan.DurationDays,
		DeliveryMode:           plan.DeliveryMode,
		CustomerName:           strings.TrimSpace(r.FormValue("customer_name")),
		Contact:                strings.TrimSpace(r.FormValue("contact")),
		DeviceLabel:            strings.TrimSpace(r.FormValue("device_label")),
		Note:                   strings.TrimSpace(r.FormValue("note")),
		ScreenshotOriginalName: header.Filename,
	}

	if order.CustomerName == "" || order.Contact == "" {
		http.Error(w, "customer_name and contact are required", http.StatusBadRequest)
		return
	}

	filename, contentType, err := s.saveScreenshot(order.ID, file, header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order.ScreenshotFile = filename
	order.ScreenshotContentType = contentType

	if err := s.fulfillOrder(&order, plan); err != nil {
		_ = os.Remove(filepath.Join(s.store.ScreenshotDir(), filename))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if _, err := s.store.Create(order); err != nil {
		_ = os.Remove(filepath.Join(s.store.ScreenshotDir(), filename))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.orderURL(r, order), http.StatusSeeOther)
}

func (s *Service) fulfillOrder(order *Order, plan PlanConfig) error {
	note := fmt.Sprintf("order=%s contact=%s", order.ID, order.Contact)
	if order.Note != "" {
		note += " note=" + order.Note
	}
	activation, err := s.controlPlane.Activate(plan.ID, buildInviteName(order.CustomerName, plan.Name), note, plan.MaxUses)
	if err != nil {
		return err
	}

	order.InviteCode = strings.TrimSpace(activation.Invite.Code)
	order.InviteRedeemURL = absoluteControlPlanePath(s.cfg.ControlPlaneBaseURL, activation.RedeemURL)
	order.InviteAPIURL = absoluteControlPlanePath(s.cfg.ControlPlaneBaseURL, activation.APIRedeemURL)
	order.PublicAPI = strings.TrimSpace(activation.PublicAPI)

	if plan.DeliveryMode != DeliveryModeProfileBundle {
		return nil
	}

	deviceID := "site-" + order.ID
	deviceName := firstNonEmpty(strings.TrimSpace(order.DeviceLabel), plan.Badge, "Website order")
	redeem, err := s.controlPlane.Redeem(order.InviteCode, deviceID, deviceName)
	if err != nil {
		return err
	}

	order.ClientID = strings.TrimSpace(redeem.Client.ID)
	order.ClientUUID = strings.TrimSpace(redeem.Client.UUID)
	order.SubscriptionURL = strings.TrimSpace(redeem.SubscriptionURL)
	order.SubscriptionText = strings.TrimSpace(redeem.SubscriptionText)
	order.PrimaryVLESSURL = strings.TrimSpace(redeem.ClientProfileVLESSURL)
	order.VLESSURLs = normalizeStringList(redeem.ClientProfilesVLESSURL)
	order.ProfileYAMLs = normalizeStringList(redeem.ClientProfilesYAML)
	return nil
}

func (s *Service) handleOrderRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/order/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	order, err := s.store.Find(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !s.orderAuthorized(r, order) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if len(parts) >= 3 && parts[1] == "profiles" && strings.HasSuffix(parts[2], ".yaml") {
		s.handleOrderProfileDownload(w, r, order, parts[2])
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.orderTpl.Execute(w, map[string]any{
		"Order":              order,
		"AndroidLauncherURL": s.cfg.AndroidLauncherURL,
		"WindowsLauncherURL": s.cfg.WindowsLauncherURL,
		"HappDownloadURL":    s.cfg.HappDownloadURL,
	})
}

func (s *Service) handleOrderProfileDownload(w http.ResponseWriter, r *http.Request, order Order, fileName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	indexText := strings.TrimSuffix(fileName, ".yaml")
	index, err := strconv.Atoi(indexText)
	if err != nil || index < 0 || index >= len(order.ProfileYAMLs) {
		http.NotFound(w, r)
		return
	}
	payload := strings.TrimSpace(order.ProfileYAMLs[index])
	if payload == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%d.yaml"`, order.ID, index+1))
	_, _ = io.WriteString(w, payload+"\n")
}

func (s *Service) handleModeratorLogin(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.AdminToken) == "" {
		http.Redirect(w, r, "/moderator/orders", http.StatusFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.renderModeratorLogin(w, "")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(r.FormValue("token")) != strings.TrimSpace(s.cfg.AdminToken) {
			s.renderModeratorLogin(w, "Неверный токен.")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     s.moderatorCookie,
			Value:    s.cfg.AdminToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/moderator/orders", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleModeratorLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.moderatorCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/moderator/login", http.StatusFound)
}

func (s *Service) redirectModeratorOrders(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/moderator/orders", http.StatusFound)
}

func (s *Service) handleModeratorOrders(w http.ResponseWriter, r *http.Request) {
	if !s.moderatorAuthorized(r) {
		http.Redirect(w, r, "/moderator/login", http.StatusFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orders, err := s.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.moderatorTpl.Execute(w, map[string]any{
		"Orders": orders,
	})
}

func (s *Service) handleModeratorOrderRoutes(w http.ResponseWriter, r *http.Request) {
	if !s.moderatorAuthorized(r) {
		http.Redirect(w, r, "/moderator/login", http.StatusFound)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/moderator/orders/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	order, err := s.store.Find(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 2 && parts[1] == "screenshot" {
		s.handleModeratorScreenshot(w, r, order)
		return
	}
	if len(parts) == 2 && parts[1] == "confirm" {
		s.handleModeratorConfirm(w, r, order)
		return
	}
	if len(parts) == 2 && parts[1] == "reject" {
		s.handleModeratorReject(w, r, order)
		return
	}
	if len(parts) != 1 || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.orderURL(r, order), http.StatusFound)
}

func (s *Service) handleModeratorScreenshot(w http.ResponseWriter, r *http.Request, order Order) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(order.ScreenshotFile) == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.store.ScreenshotDir(), order.ScreenshotFile)
	w.Header().Set("Content-Type", firstNonEmpty(order.ScreenshotContentType, "application/octet-stream"))
	http.ServeFile(w, r, path)
}

func (s *Service) handleModeratorConfirm(w http.ResponseWriter, r *http.Request, order Order) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, err := s.store.Update(order.ID, func(target *Order) error {
		now := time.Now().UTC()
		target.Status = OrderStatusConfirmed
		target.ReviewReason = ""
		target.ReviewedBy = "moderator"
		target.ReviewedAt = &now
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/moderator/orders", http.StatusSeeOther)
}

func (s *Service) handleModeratorReject(w http.ResponseWriter, r *http.Request, order Order) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		reason = "payment not confirmed"
	}

	if err := s.controlPlane.Reject(order.ClientID, order.InviteCode, reason, "pay-service"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	_, err := s.store.Update(order.ID, func(target *Order) error {
		now := time.Now().UTC()
		target.Status = OrderStatusRejected
		target.ReviewReason = reason
		target.ReviewedBy = "moderator"
		target.ReviewedAt = &now
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/moderator/orders", http.StatusSeeOther)
}

func (s *Service) renderModeratorLogin(w http.ResponseWriter, notice string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.loginTpl.Execute(w, map[string]any{"Notice": notice})
}

func (s *Service) moderatorAuthorized(r *http.Request) bool {
	if strings.TrimSpace(s.cfg.AdminToken) == "" {
		return true
	}
	if header := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:]) == strings.TrimSpace(s.cfg.AdminToken)
	}
	if cookie, err := r.Cookie(s.moderatorCookie); err == nil {
		return cookie.Value == strings.TrimSpace(s.cfg.AdminToken)
	}
	return false
}

func (s *Service) orderAuthorized(r *http.Request, order Order) bool {
	if s.moderatorAuthorized(r) {
		return true
	}
	return strings.TrimSpace(r.URL.Query().Get("token")) == strings.TrimSpace(order.AccessToken)
}

func (s *Service) orderURL(r *http.Request, order Order) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.PublicBaseURL), "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return fmt.Sprintf("%s/order/%s?token=%s", base, order.ID, order.AccessToken)
}

func (s *Service) saveScreenshot(orderID string, file multipart.File, header *multipart.FileHeader) (string, string, error) {
	if err := os.MkdirAll(s.store.ScreenshotDir(), 0o755); err != nil {
		return "", "", err
	}

	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return "", "", err
	}
	contentType := http.DetectContentType(sniff[:n])
	extension := screenshotExtension(contentType)
	if extension == "" {
		return "", "", fmt.Errorf("unsupported screenshot type")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}

	filename := orderID + extension
	path := filepath.Join(s.store.ScreenshotDir(), filename)
	output, err := os.Create(path)
	if err != nil {
		return "", "", err
	}
	defer output.Close()

	limited := io.LimitReader(file, 10<<20)
	if _, err := io.Copy(output, limited); err != nil {
		return "", "", err
	}
	return filename, contentType, nil
}

func screenshotExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func buildInviteName(customerName string, planName string) string {
	name := strings.TrimSpace(customerName)
	if name == "" {
		name = "customer"
	}
	return fmt.Sprintf("%s · %s", name, strings.TrimSpace(planName))
}

func absoluteControlPlanePath(base string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	base = strings.TrimSuffix(base, "/admin")
	return base + path
}

func statusLabel(status OrderStatus) string {
	switch status {
	case OrderStatusConfirmed:
		return "Оплата подтверждена"
	case OrderStatusRejected:
		return "Отклонено"
	default:
		return "Ждёт модерации"
	}
}

func formatMoney(minor int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "RUB", "":
		return fmt.Sprintf("%d ₽ / месяц", minor)
	default:
		return fmt.Sprintf("%d %s", minor, currency)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
