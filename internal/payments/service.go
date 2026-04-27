package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	errOrderAlreadyActive = errors.New("order is already active")
	errOrderProvisioning  = errors.New("order is provisioning")
	errOrderCancelled     = errors.New("order is cancelled")
)

type Service struct {
	cfg             Config
	store           *OrderStore
	controlPlane    *ControlPlaneClient
	logger          *slog.Logger
	httpServer      *http.Server
	landingTpl      *template.Template
	orderTpl        *template.Template
	accountTpl      *template.Template
	loginTpl        *template.Template
	moderatorTpl    *template.Template
	moderatorCookie string
}

type PricingQuote struct {
	DeviceCount           int    `json:"device_count"`
	Months                int    `json:"months"`
	DiscountPercent       int    `json:"discount_percent"`
	BaseMonthlyPriceMinor int64  `json:"base_monthly_price_minor"`
	SubtotalMinor         int64  `json:"subtotal_minor"`
	DiscountMinor         int64  `json:"discount_minor"`
	PromoDiscountMinor    int64  `json:"promo_discount_minor"`
	TotalMinor            int64  `json:"total_minor"`
	DurationDays          int    `json:"duration_days"`
	Currency              string `json:"currency"`
}

type landingView struct {
	BrandName          string
	SupportLink        string
	AndroidLauncherURL string
	WindowsLauncherURL string
	HappDownloadURL    string
	Pricing            PricingConfig
	DefaultQuote       PricingQuote
	AccountID          string
	AccountToken       string
	CustomerName       string
	Contact            string
	DeviceLabel        string
	Note               string
	PromoCode          string
	SiteKeyNotice      string
}

type orderView struct {
	BrandName     string
	Order         Order
	AccountURL    string
	AndroidURL    string
	WindowsURL    string
	HappURL       string
	SBPCardNumber string
	SBPCardHolder string
	SBPCardBank   string
}

type accountView struct {
	BrandName          string
	Pricing            PricingConfig
	AccountID          string
	AccountToken       string
	AccountURL         string
	CustomerName       string
	Contact            string
	PromoCode          string
	Orders             []Order
	ActiveKeys         []accountKeyView
	DefaultQuote       PricingQuote
	AndroidLauncherURL string
	WindowsLauncherURL string
	HappDownloadURL    string
	SupportLink        string
}

type moderatorView struct {
	BrandName string
	Orders    []Order
	Promos    []PromoCode
}

type accountKeyView struct {
	OrderID      string
	OrderURL     string
	Status       OrderStatus
	SlotNumber   int
	Label        string
	ClientID     string
	ClientUUID   string
	Subscription string
	VLESS        string
	IssuedAt     time.Time
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
	}

	funcs := template.FuncMap{
		"formatMoney": func(minor int64, currency string) string {
			return formatMoney(minor, currency)
		},
		"statusLabel": localizedStatusLabel,
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
		"toJSON": func(value any) template.JS {
			body, err := json.Marshal(value)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(body)
		},
	}

	service.landingTpl = template.Must(template.New("landing").Funcs(funcs).Parse(landingTemplate))
	service.orderTpl = template.Must(template.New("order").Funcs(funcs).Parse(orderTemplate))
	service.accountTpl = template.Must(template.New("account").Funcs(funcs).Parse(accountTemplate))
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
	mux.HandleFunc("/s/", s.handleCompatShortSubscription)
	mux.HandleFunc("/redeem/", s.handleCompatPublicControlPlane)
	mux.HandleFunc("/disconnect", s.handleCompatPublicControlPlane)
	mux.HandleFunc("/client/policy", s.handleCompatPublicControlPlane)
	mux.HandleFunc("/client/subscription", s.handleCompatPublicControlPlane)
	mux.HandleFunc("/client/notices", s.handleCompatPublicControlPlane)
	mux.HandleFunc("/client/quota", s.handleCompatPublicControlPlane)
	mux.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(s.downloadsDir()))))
	mux.HandleFunc("/cabinet/open", s.handleCabinetOpen)
	mux.HandleFunc("/cabinet/", s.handleCabinetRoutes)
	mux.HandleFunc("/order/", s.handleOrderRoutes)
	mux.HandleFunc("/order", s.handleCreateOrder)
	mux.HandleFunc("/moderator/login", s.handleModeratorLogin)
	mux.HandleFunc("/moderator/logout", s.handleModeratorLogout)
	mux.HandleFunc("/moderator/orders/", s.handleModeratorOrderRoutes)
	mux.HandleFunc("/moderator/promos", s.handleModeratorPromos)
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
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Service) handleCompatShortSubscription(w http.ResponseWriter, r *http.Request) {
	clientUUID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/s/"), "/")
	if clientUUID == "" {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	query.Set("client_uuid", clientUUID)
	targetPath := "/client/subscription?" + query.Encode()
	s.proxyPublicControlPlane(w, r, targetPath)
}

func (s *Service) handleCompatPublicControlPlane(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Path
	if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
		targetPath += "?" + rawQuery
	}
	s.proxyPublicControlPlane(w, r, targetPath)
}

func (s *Service) proxyPublicControlPlane(w http.ResponseWriter, r *http.Request, targetPath string) {
	targetURL, err := s.compatControlPlaneURL(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	copyProxyHeaders(req.Header, r.Header)

	resp, err := s.controlPlane.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyProxyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Service) compatControlPlaneURL(targetPath string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.ControlPlaneBaseURL), "/")
	if base == "" {
		return "", errors.New("control plane base URL is empty")
	}
	if strings.HasPrefix(targetPath, "http://") || strings.HasPrefix(targetPath, "https://") {
		return targetPath, nil
	}
	if strings.HasPrefix(targetPath, "/") {
		return base + targetPath, nil
	}
	return base + "/" + targetPath, nil
}

func copyProxyHeaders(target http.Header, source http.Header) {
	for key, values := range source {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "connection", "content-length", "host", "transfer-encoding":
			continue
		}
		target.Del(key)
		for _, value := range values {
			target.Add(key, value)
		}
	}
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
	defaultQuote, err := s.quoteForSelection(s.cfg.Pricing.DefaultDevices, s.cfg.Pricing.DefaultMonths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := landingView{
		BrandName:          s.cfg.BrandName,
		SupportLink:        s.cfg.SupportLink,
		AndroidLauncherURL: s.cfg.AndroidLauncherURL,
		WindowsLauncherURL: s.cfg.WindowsLauncherURL,
		HappDownloadURL:    s.cfg.HappDownloadURL,
		Pricing:            s.cfg.Pricing,
		DefaultQuote:       defaultQuote,
		AccountID:          strings.TrimSpace(r.URL.Query().Get("account_id")),
		AccountToken:       strings.TrimSpace(r.URL.Query().Get("account_token")),
		CustomerName:       strings.TrimSpace(r.URL.Query().Get("customer_name")),
		Contact:            strings.TrimSpace(r.URL.Query().Get("contact")),
		DeviceLabel:        strings.TrimSpace(r.URL.Query().Get("device_label")),
		Note:               strings.TrimSpace(r.URL.Query().Get("note")),
		PromoCode:          strings.TrimSpace(r.URL.Query().Get("promo_code")),
	}
	if view.AccountID != "" && view.AccountToken != "" {
		view.SiteKeyNotice = "Включен режим продления: новый заказ будет привязан к существующему кабинету."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.landingTpl.Execute(w, view)
}

func (s *Service) handleCabinetOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	siteKey := strings.TrimSpace(r.FormValue("site_key"))
	accountID, _, err := s.store.FindByAccountToken(siteKey)
	if err != nil {
		http.Error(w, "кабинет не найден", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, s.accountURLFromValues(accountID, siteKey), http.StatusSeeOther)
}

func (s *Service) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	customerName := strings.TrimSpace(r.FormValue("customer_name"))
	contact := strings.TrimSpace(r.FormValue("contact"))
	if customerName == "" || contact == "" {
		http.Error(w, "имя и контакт обязательны", http.StatusBadRequest)
		return
	}

	deviceCount, months, err := parseDeviceAndTerm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	quote, err := s.quoteForSelection(deviceCount, months)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	promoCode := strings.TrimSpace(r.FormValue("promo_code"))
	var promo PromoCode
	if promoCode != "" {
		promo, err = s.store.FindPromo(promoCode)
		if err != nil {
			http.Error(w, "промокод не найден", http.StatusBadRequest)
			return
		}
		now := time.Now().UTC()
		if !promo.isRedeemable(now) {
			http.Error(w, promo.redeemError(now).Error(), http.StatusBadRequest)
			return
		}
		quote = applyPromoToQuote(quote, promo)
	}

	accountID := strings.TrimSpace(r.FormValue("account_id"))
	accountToken := strings.TrimSpace(r.FormValue("account_token"))
	if accountID != "" || accountToken != "" {
		if err := s.assertAccount(accountID, accountToken); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	} else {
		accountID, err = newAccountID()
		if err != nil {
			http.Error(w, "failed to create account id", http.StatusInternalServerError)
			return
		}
		accountToken, err = newAccountToken()
		if err != nil {
			http.Error(w, "failed to create account token", http.StatusInternalServerError)
			return
		}
	}

	orderID, err := newOrderID()
	if err != nil {
		http.Error(w, "failed to create order id", http.StatusInternalServerError)
		return
	}
	accessToken, err := newAccessToken()
	if err != nil {
		http.Error(w, "failed to create order token", http.StatusInternalServerError)
		return
	}

	order := Order{
		ID:                    orderID,
		AccountID:             accountID,
		AccountToken:          accountToken,
		AccessToken:           accessToken,
		Status:                OrderStatusAwaitingPayment,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
		PlanID:                strings.TrimSpace(s.cfg.Pricing.PlanID),
		PlanName:              localizedPlanName(s.cfg.Pricing.ProductName, deviceCount, months),
		PlanBadge:             "СБП",
		PlanDescription:       strings.TrimSpace(s.cfg.Pricing.ProductDescription),
		PriceMinor:            quote.TotalMinor,
		Currency:              quote.Currency,
		DurationDays:          quote.DurationDays,
		DeliveryMode:          DeliveryModeProfileBundle,
		CustomerName:          customerName,
		Contact:               contact,
		DeviceLabel:           strings.TrimSpace(r.FormValue("device_label")),
		DeviceCount:           quote.DeviceCount,
		Months:                quote.Months,
		DiscountPercent:       quote.DiscountPercent,
		BaseMonthlyPriceMinor: quote.BaseMonthlyPriceMinor,
		SubtotalMinor:         quote.SubtotalMinor,
		DiscountMinor:         quote.DiscountMinor,
		PromoCode:             strings.TrimSpace(promo.Code),
		PromoName:             strings.TrimSpace(promo.Name),
		PromoDiscountPercent:  promo.DiscountPercent,
		PromoDiscountMinor:    quote.PromoDiscountMinor,
		Note:                  strings.TrimSpace(r.FormValue("note")),
	}

	if _, err := s.store.Create(order); err != nil {
		statusCode := http.StatusInternalServerError
		if strings.TrimSpace(order.PromoCode) != "" {
			statusCode = http.StatusBadRequest
		}
		http.Error(w, err.Error(), statusCode)
		return
	}
	if order.PriceMinor == 0 {
		if err := s.activateZeroCostOrder(order); err != nil {
			s.logger.Error("activate zero cost order", "order_id", order.ID, "error", err)
		}
	}
	http.Redirect(w, r, s.orderURL(r, order), http.StatusSeeOther)
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

	if len(parts) == 2 && parts[1] == "activate" {
		s.handleOrderActivate(w, r, order)
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
	_ = s.orderTpl.Execute(w, orderView{
		BrandName:     s.cfg.BrandName,
		Order:         order,
		AccountURL:    s.accountURL(order),
		AndroidURL:    s.cfg.AndroidLauncherURL,
		WindowsURL:    s.cfg.WindowsLauncherURL,
		HappURL:       s.cfg.HappDownloadURL,
		SBPCardNumber: s.cfg.PaymentCardNumber,
		SBPCardHolder: s.cfg.PaymentCardHolder,
		SBPCardBank:   s.cfg.PaymentCardBank,
	})
}

func (s *Service) handleOrderActivate(w http.ResponseWriter, r *http.Request, order Order) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.activateExistingOrder(order); err != nil {
		switch err {
		case errOrderAlreadyActive, errOrderProvisioning:
			http.Redirect(w, r, s.orderURL(r, order), http.StatusSeeOther)
			return
		case errOrderCancelled:
			http.Error(w, err.Error(), http.StatusConflict)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, s.orderURL(r, order), http.StatusSeeOther)
}

func (s *Service) activateZeroCostOrder(order Order) error {
	return s.activateExistingOrder(order)
}

func (s *Service) activateExistingOrder(order Order) error {

	order, err := s.store.Update(order.ID, func(target *Order) error {
		switch target.Status {
		case OrderStatusActive:
			return errOrderAlreadyActive
		case OrderStatusProvisioning:
			return errOrderProvisioning
		case OrderStatusCancelled:
			return errOrderCancelled
		default:
			target.Status = OrderStatusProvisioning
			target.ProvisionError = ""
			return nil
		}
	})
	if err != nil {
		return err
	}

	provisioned, err := s.provisionOrder(order)
	if err != nil {
		_, _ = s.store.Update(order.ID, func(target *Order) error {
			target.Status = OrderStatusAwaitingPayment
			target.ProvisionError = err.Error()
			return nil
		})
		return err
	}

	now := time.Now().UTC()
	_, err = s.store.Update(order.ID, func(target *Order) error {
		target.Status = OrderStatusActive
		target.ActivatedAt = &now
		target.ProvisionError = ""
		target.InviteCode = provisioned.InviteCode
		target.InviteRedeemURL = provisioned.InviteRedeemURL
		target.InviteAPIURL = provisioned.InviteAPIURL
		target.PublicAPI = provisioned.PublicAPI
		target.ClientID = provisioned.ClientID
		target.ClientUUID = provisioned.ClientUUID
		target.SubscriptionURL = provisioned.SubscriptionURL
		target.SubscriptionText = provisioned.SubscriptionText
		target.PrimaryVLESSURL = provisioned.PrimaryVLESSURL
		target.VLESSURLs = append([]string(nil), provisioned.VLESSURLs...)
		target.ProfileYAMLs = append([]string(nil), provisioned.ProfileYAMLs...)
		target.AccessKeys = append([]AccessKey(nil), provisioned.AccessKeys...)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) provisionOrder(order Order) (Order, error) {
	activation, err := s.controlPlane.Activate(ActivationRequest{
		PlanID:             order.PlanID,
		Name:               localizedInviteName(order.CustomerName, order.PlanName),
		Note:               s.orderProvisionNote(order),
		MaxUses:            maxInt(order.DeviceCount, 1),
		AccessDurationDays: order.DurationDays,
	})
	if err != nil {
		return Order{}, err
	}

	order.InviteCode = strings.TrimSpace(activation.Invite.Code)
	order.InviteRedeemURL = absoluteControlPlanePath(s.cfg.ControlPlaneBaseURL, activation.RedeemURL)
	order.InviteAPIURL = absoluteControlPlanePath(s.cfg.ControlPlaneBaseURL, activation.APIRedeemURL)
	order.PublicAPI = strings.TrimSpace(activation.PublicAPI)

	keys := make([]AccessKey, 0, maxInt(order.DeviceCount, 1))
	for slot := 1; slot <= maxInt(order.DeviceCount, 1); slot++ {
		label := s.accessKeyLabel(order, slot)
		deviceID := fmt.Sprintf("site-%s-%d", order.ID, slot)
		redeem, err := s.controlPlane.Redeem(order.InviteCode, deviceID, label)
		if err != nil {
			return Order{}, err
		}
		key := AccessKey{
			SlotNumber:       slot,
			Label:            label,
			DeviceID:         deviceID,
			ClientID:         strings.TrimSpace(redeem.Client.ID),
			ClientUUID:       strings.TrimSpace(redeem.Client.UUID),
			SubscriptionURL:  strings.TrimSpace(redeem.SubscriptionURL),
			SubscriptionText: strings.TrimSpace(redeem.SubscriptionText),
			PrimaryVLESSURL:  strings.TrimSpace(redeem.ClientProfileVLESSURL),
			VLESSURLs:        normalizeStringList(redeem.ClientProfilesVLESSURL),
			ProfileYAMLs:     normalizeStringList(redeem.ClientProfilesYAML),
			IssuedAt:         time.Now().UTC(),
		}
		keys = append(keys, key)
	}
	order.AccessKeys = keys
	if len(keys) > 0 {
		first := keys[0]
		order.ClientID = first.ClientID
		order.ClientUUID = first.ClientUUID
		order.SubscriptionURL = first.SubscriptionURL
		order.SubscriptionText = first.SubscriptionText
		order.PrimaryVLESSURL = first.PrimaryVLESSURL
		order.VLESSURLs = append([]string(nil), first.VLESSURLs...)
		order.ProfileYAMLs = append([]string(nil), first.ProfileYAMLs...)
	}
	return order, nil
}

func (s *Service) handleCabinetRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/cabinet/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orders, err := s.store.FindByAccount(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !s.accountAuthorized(r, orders) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	defaultDevices := s.cfg.Pricing.DefaultDevices
	defaultMonths := s.cfg.Pricing.DefaultMonths
	customerName := ""
	contact := ""
	if len(orders) > 0 {
		defaultDevices = maxInt(orders[0].DeviceCount, defaultDevices)
		defaultMonths = maxInt(orders[0].Months, defaultMonths)
		customerName = orders[0].CustomerName
		contact = orders[0].Contact
	}
	defaultQuote, err := s.quoteForSelection(defaultDevices, defaultMonths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	accountToken := strings.TrimSpace(orders[0].AccountToken)
	view := accountView{
		BrandName:          s.cfg.BrandName,
		Pricing:            s.cfg.Pricing,
		AccountID:          path,
		AccountToken:       accountToken,
		AccountURL:         s.accountURLFromValues(path, accountToken),
		CustomerName:       customerName,
		Contact:            contact,
		PromoCode:          strings.TrimSpace(r.URL.Query().Get("promo_code")),
		Orders:             orders,
		ActiveKeys:         s.accountKeyViews(r, orders),
		DefaultQuote:       defaultQuote,
		AndroidLauncherURL: s.cfg.AndroidLauncherURL,
		WindowsLauncherURL: s.cfg.WindowsLauncherURL,
		HappDownloadURL:    s.cfg.HappDownloadURL,
		SupportLink:        s.cfg.SupportLink,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.accountTpl.Execute(w, view)
}

func (s *Service) accountKeyViews(r *http.Request, orders []Order) []accountKeyView {
	result := make([]accountKeyView, 0)
	for _, order := range orders {
		if order.Status != OrderStatusActive {
			continue
		}
		for _, key := range order.AccessKeys {
			result = append(result, accountKeyView{
				OrderID:      order.ID,
				OrderURL:     s.orderURL(r, order),
				Status:       order.Status,
				SlotNumber:   key.SlotNumber,
				Label:        key.Label,
				ClientID:     key.ClientID,
				ClientUUID:   key.ClientUUID,
				Subscription: key.SubscriptionURL,
				VLESS:        firstNonEmpty(key.PrimaryVLESSURL, firstString(key.VLESSURLs)),
				IssuedAt:     key.IssuedAt,
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].IssuedAt.Equal(result[j].IssuedAt) {
			return result[i].OrderID > result[j].OrderID
		}
		return result[i].IssuedAt.After(result[j].IssuedAt)
	})
	return result
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
			s.renderModeratorLogin(w, "Неверный токен модератора.")
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
	promos, err := s.store.ListPromos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.moderatorTpl.Execute(w, moderatorView{
		BrandName: s.cfg.BrandName,
		Orders:    orders,
		Promos:    promos,
	})
}

func (s *Service) handleModeratorOrderRoutes(w http.ResponseWriter, r *http.Request) {
	if !s.moderatorAuthorized(r) {
		http.Redirect(w, r, "/moderator/login", http.StatusFound)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/moderator/orders/")
	order, err := s.store.Find(strings.Trim(path, "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.orderURL(r, order), http.StatusFound)
}

func (s *Service) handleModeratorPromos(w http.ResponseWriter, r *http.Request) {
	if !s.moderatorAuthorized(r) {
		http.Redirect(w, r, "/moderator/login", http.StatusFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	discountPercent, err := parsePositiveIntAllowZero(r.FormValue("discount_percent"))
	if err != nil {
		http.Error(w, "некорректный процент скидки", http.StatusBadRequest)
		return
	}
	maxUses, err := parsePositiveIntAllowZero(r.FormValue("max_uses"))
	if err != nil {
		http.Error(w, "некорректный лимит использований", http.StatusBadRequest)
		return
	}
	expiresHours, err := parsePositiveIntAllowZero(r.FormValue("expires_in_hours"))
	if err != nil {
		http.Error(w, "некорректный срок действия", http.StatusBadRequest)
		return
	}
	_, err = s.store.CreatePromo(PromoCreateRequest{
		Code:            r.FormValue("code"),
		Name:            r.FormValue("name"),
		DiscountPercent: discountPercent,
		MaxUses:         maxUses,
		ExpiresAfter:    time.Duration(expiresHours) * time.Hour,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

func (s *Service) accountAuthorized(r *http.Request, orders []Order) bool {
	if s.moderatorAuthorized(r) {
		return true
	}
	if len(orders) == 0 {
		return false
	}
	return strings.TrimSpace(r.URL.Query().Get("token")) == strings.TrimSpace(orders[0].AccountToken)
}

func (s *Service) assertAccount(accountID string, accountToken string) error {
	orders, err := s.store.FindByAccount(accountID)
	if err != nil {
		return errors.New("кабинет не найден")
	}
	if len(orders) == 0 || strings.TrimSpace(orders[0].AccountToken) != strings.TrimSpace(accountToken) {
		return errors.New("неверный ключ кабинета")
	}
	return nil
}

func (s *Service) orderURL(r *http.Request, order Order) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.PublicBaseURL), "/")
	if base == "" {
		scheme := "http"
		if r != nil && r.TLS != nil {
			scheme = "https"
		}
		host := ""
		if r != nil {
			host = r.Host
		}
		base = scheme + "://" + host
	}
	return fmt.Sprintf("%s/order/%s?token=%s", base, order.ID, order.AccessToken)
}

func (s *Service) accountURL(order Order) string {
	return s.accountURLFromValues(order.AccountID, order.AccountToken)
}

func (s *Service) accountURLFromValues(accountID string, accountToken string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.PublicBaseURL), "/")
	if base == "" {
		base = ""
	}
	return fmt.Sprintf("%s/cabinet/%s?token=%s", base, strings.TrimSpace(accountID), strings.TrimSpace(accountToken))
}

func (s *Service) quoteForSelection(deviceCount int, months int) (PricingQuote, error) {
	if deviceCount < s.cfg.Pricing.MinDevices || deviceCount > s.cfg.Pricing.MaxDevices {
		return PricingQuote{}, fmt.Errorf("количество устройств должно быть в диапазоне %d..%d", s.cfg.Pricing.MinDevices, s.cfg.Pricing.MaxDevices)
	}
	option, ok := s.monthOption(months)
	if !ok {
		return PricingQuote{}, errors.New("неподдерживаемый срок подписки")
	}
	subtotal := s.cfg.Pricing.BaseMonthlyPriceMinor * int64(deviceCount) * int64(months)
	discount := subtotal * int64(option.DiscountPercent) / 100
	return PricingQuote{
		DeviceCount:           deviceCount,
		Months:                months,
		DiscountPercent:       option.DiscountPercent,
		BaseMonthlyPriceMinor: s.cfg.Pricing.BaseMonthlyPriceMinor,
		SubtotalMinor:         subtotal,
		DiscountMinor:         discount,
		PromoDiscountMinor:    0,
		TotalMinor:            subtotal - discount,
		DurationDays:          months * 30,
		Currency:              firstNonEmpty(strings.TrimSpace(s.cfg.Pricing.Currency), "RUB"),
	}, nil
}

func applyPromoToQuote(quote PricingQuote, promo PromoCode) PricingQuote {
	baseAfterTermDiscount := quote.SubtotalMinor - quote.DiscountMinor
	if baseAfterTermDiscount < 0 {
		baseAfterTermDiscount = 0
	}
	quote.PromoDiscountMinor = baseAfterTermDiscount * int64(promo.DiscountPercent) / 100
	quote.TotalMinor = baseAfterTermDiscount - quote.PromoDiscountMinor
	if quote.TotalMinor < 0 {
		quote.TotalMinor = 0
	}
	return quote
}

func (s *Service) monthOption(months int) (PricingMonthOption, bool) {
	for _, option := range s.cfg.Pricing.MonthOptions {
		if option.Months == months {
			return option, true
		}
	}
	return PricingMonthOption{}, false
}

func (s *Service) orderProvisionNote(order Order) string {
	parts := []string{
		"order=" + order.ID,
		"account=" + order.AccountID,
		"contact=" + order.Contact,
		fmt.Sprintf("devices=%d", order.DeviceCount),
		fmt.Sprintf("months=%d", order.Months),
	}
	if order.Note != "" {
		parts = append(parts, "note="+order.Note)
	}
	return strings.Join(parts, " ")
}

func (s *Service) accessKeyLabel(order Order, slot int) string {
	if order.DeviceCount <= 1 && strings.TrimSpace(order.DeviceLabel) != "" {
		return strings.TrimSpace(order.DeviceLabel)
	}
	if strings.TrimSpace(order.DeviceLabel) != "" {
		return fmt.Sprintf("%s #%d", strings.TrimSpace(order.DeviceLabel), slot)
	}
	return fmt.Sprintf("Устройство %d", slot)
}

func parseDeviceAndTerm(r *http.Request) (int, int, error) {
	deviceCount, err := parsePositiveInt(r.FormValue("device_count"))
	if err != nil {
		return 0, 0, errors.New("некорректное количество устройств")
	}
	months, err := parsePositiveInt(r.FormValue("months"))
	if err != nil {
		return 0, 0, errors.New("некорректный срок подписки")
	}
	return deviceCount, months, nil
}

func parsePositiveIntAllowZero(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	var result int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, errors.New("non numeric")
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}

func parsePositiveInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("empty value")
	}
	var result int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, errors.New("non numeric")
		}
		result = result*10 + int(r-'0')
	}
	if result <= 0 {
		return 0, errors.New("must be positive")
	}
	return result, nil
}

func buildOrderPlanName(productName string, deviceCount int, months int) string {
	return fmt.Sprintf("%s · %d device(s) · %d month(s)", strings.TrimSpace(productName), deviceCount, months)
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

func localizedPlanName(productName string, deviceCount int, months int) string {
	return fmt.Sprintf("%s · %d устройств · %d мес.", strings.TrimSpace(productName), deviceCount, months)
}

func localizedInviteName(customerName string, planName string) string {
	name := strings.TrimSpace(customerName)
	if name == "" {
		name = "клиент"
	}
	return fmt.Sprintf("%s · %s", name, strings.TrimSpace(planName))
}

func localizedStatusLabel(status OrderStatus) string {
	switch status {
	case OrderStatusActive:
		return "Доступ выдан"
	case OrderStatusProvisioning:
		return "Выдаем ключи"
	case OrderStatusCancelled:
		return "Отменено"
	default:
		return "Ожидает оплаты"
	}
}

func statusLabel(status OrderStatus) string {
	switch status {
	case OrderStatusActive:
		return "Access issued"
	case OrderStatusProvisioning:
		return "Provisioning keys"
	case OrderStatusCancelled:
		return "Cancelled"
	default:
		return "Waiting for payment"
	}
}

func formatMoney(minor int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "RUB", "":
		return fmt.Sprintf("%d ₽", minor)
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

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
