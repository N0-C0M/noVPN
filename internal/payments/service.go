package payments

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr          string        `yaml:"listen_addr"`
	StoragePath         string        `yaml:"storage_path"`
	AdminToken          string        `yaml:"admin_token"`
	PublicBaseURL       string        `yaml:"public_base_url"`
	ControlPlaneBaseURL string        `yaml:"control_plane_base_url"`
	ControlPlaneToken   string        `yaml:"control_plane_token"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
}

type Plan struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	DurationDays      int    `json:"duration_days,omitempty"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes,omitempty"`
	PriceMinor        int64  `json:"price_minor,omitempty"`
	Currency          string `json:"currency,omitempty"`
}

type Order struct {
	ID              string     `json:"id"`
	PlanID          string     `json:"plan_id"`
	PlanName        string     `json:"plan_name"`
	CustomerName    string     `json:"customer_name"`
	CustomerContact string     `json:"customer_contact"`
	Status          string     `json:"status"`
	InviteCode      string     `json:"invite_code,omitempty"`
	RedeemURL       string     `json:"redeem_url,omitempty"`
	CheckoutURL     string     `json:"checkout_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	ActivatedAt     *time.Time `json:"activated_at,omitempty"`
}

type orderStore struct {
	path string
	mu   sync.Mutex
}

type Service struct {
	cfg    Config
	server *http.Server
	store  *orderStore
	client *http.Client
	ln     net.Listener
}

func LoadConfig(path string) (Config, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(payload, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		cfg.ListenAddr = "127.0.0.1:9120"
	}
	if strings.TrimSpace(cfg.StoragePath) == "" {
		cfg.StoragePath = "/var/lib/novpn/payments/orders.json"
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 15 * time.Second
	}
	if strings.TrimSpace(cfg.ControlPlaneBaseURL) == "" {
		return Config{}, fmt.Errorf("control_plane_base_url must not be empty")
	}
	if strings.TrimSpace(cfg.ControlPlaneToken) == "" {
		return Config{}, fmt.Errorf("control_plane_token must not be empty")
	}
	return cfg, nil
}

func New(cfg Config) *Service {
	service := &Service{
		cfg:   cfg,
		store: &orderStore{path: cfg.StoragePath},
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
	service.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: service.routes(),
	}
	return service
}

func (s *Service) Start() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	s.ln = ln
	go s.server.Serve(ln)
	return nil
}

func (s *Service) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/plans", s.handlePlans)
	mux.HandleFunc("/checkout", s.handleCheckout)
	mux.HandleFunc("/orders/", s.handleOrder)
	mux.HandleFunc("/api/orders/", s.handleOrderAdmin)
	return mux
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Service) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	plans, err := s.fetchPlans(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service": "novpn-pay-service",
		"plans":   plans,
	})
}

func (s *Service) handlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plans, err := s.fetchPlans(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"observed_at": time.Now().UTC(),
		"plans":       plans,
	})
}

func (s *Service) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		PlanID          string `json:"plan_id"`
		CustomerName    string `json:"customer_name"`
		CustomerContact string `json:"customer_contact"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	plans, err := s.fetchPlans(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	plan, ok := findPlan(plans, payload.PlanID)
	if !ok {
		http.Error(w, "plan not found", http.StatusBadRequest)
		return
	}
	order := Order{
		ID:              generateID("order"),
		PlanID:          plan.ID,
		PlanName:        plan.Name,
		CustomerName:    strings.TrimSpace(payload.CustomerName),
		CustomerContact: strings.TrimSpace(payload.CustomerContact),
		Status:          "pending",
		CreatedAt:       time.Now().UTC(),
	}
	if base := strings.TrimRight(strings.TrimSpace(s.cfg.PublicBaseURL), "/"); base != "" {
		order.CheckoutURL = base + "/orders/" + order.ID
	}
	if err := s.store.append(order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Service) handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	orderID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/orders/"), "/")
	order, err := s.store.find(orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Service) handleOrderAdmin(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.cfg.AdminToken) == "" || strings.TrimSpace(r.Header.Get("X-Pay-Admin-Token")) != strings.TrimSpace(s.cfg.AdminToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/orders/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "mark-paid" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	order, err := s.store.find(parts[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if order.Status == "activated" {
		writeJSON(w, http.StatusOK, order)
		return
	}
	activation, err := s.activateOrder(r.Context(), order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	now := time.Now().UTC()
	order.Status = "activated"
	order.PaidAt = &now
	order.ActivatedAt = &now
	order.InviteCode = activation.Invite.Code
	order.RedeemURL = activation.RedeemURL
	if err := s.store.upsert(order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Service) fetchPlans(ctx context.Context) ([]Plan, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(s.cfg.ControlPlaneBaseURL), "/")+"/public/plans", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch plans returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var root struct {
		Plans []Plan `json:"plans"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, err
	}
	return root.Plans, nil
}

func (s *Service) activateOrder(ctx context.Context, order Order) (struct {
	Invite struct {
		Code string `json:"code"`
	} `json:"invite"`
	RedeemURL string `json:"redeem_url"`
}, error) {
	requestBody, err := json.Marshal(map[string]any{
		"plan_id":  order.PlanID,
		"name":     firstNonEmpty(order.CustomerName, order.PlanName),
		"note":     firstNonEmpty(order.CustomerContact, order.ID),
		"max_uses": 1,
	})
	if err != nil {
		return struct {
			Invite struct {
				Code string `json:"code"`
			} `json:"invite"`
			RedeemURL string `json:"redeem_url"`
		}{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(s.cfg.ControlPlaneBaseURL), "/")+"/control-plane/payments/activate", bytes.NewReader(requestBody))
	if err != nil {
		return struct {
			Invite struct {
				Code string `json:"code"`
			} `json:"invite"`
			RedeemURL string `json:"redeem_url"`
		}{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Control-Plane-Token", s.cfg.ControlPlaneToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return struct {
			Invite struct {
				Code string `json:"code"`
			} `json:"invite"`
			RedeemURL string `json:"redeem_url"`
		}{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return struct {
			Invite struct {
				Code string `json:"code"`
			} `json:"invite"`
			RedeemURL string `json:"redeem_url"`
		}{}, fmt.Errorf("activation returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var result struct {
		Invite struct {
			Code string `json:"code"`
		} `json:"invite"`
		RedeemURL string `json:"redeem_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func findPlan(plans []Plan, planID string) (Plan, bool) {
	for _, plan := range plans {
		if plan.ID == strings.TrimSpace(planID) {
			return plan, true
		}
	}
	return Plan{}, false
}

func generateID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().Unix())
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *orderStore) load() ([]Order, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil, nil
	}
	var orders []Order
	if err := json.Unmarshal(payload, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *orderStore) save(orders []Order) error {
	payload, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, payload, 0o600)
}

func (s *orderStore) append(order Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders, err := s.load()
	if err != nil {
		return err
	}
	orders = append(orders, order)
	return s.save(orders)
}

func (s *orderStore) find(orderID string) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders, err := s.load()
	if err != nil {
		return Order{}, err
	}
	for _, order := range orders {
		if order.ID == orderID {
			return order, nil
		}
	}
	return Order{}, fmt.Errorf("order not found")
}

func (s *orderStore) upsert(order Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	orders, err := s.load()
	if err != nil {
		return err
	}
	for index := range orders {
		if orders[index].ID == order.ID {
			orders[index] = order
			return s.save(orders)
		}
	}
	orders = append(orders, order)
	return s.save(orders)
}
