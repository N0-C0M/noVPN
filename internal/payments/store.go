package payments

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type DeliveryMode string

const (
	DeliveryModeInviteCode    DeliveryMode = "invite_code"
	DeliveryModeProfileBundle DeliveryMode = "profile_bundle"
)

type OrderStatus string

const (
	OrderStatusAwaitingPayment OrderStatus = "awaiting_payment"
	OrderStatusProvisioning    OrderStatus = "provisioning"
	OrderStatusActive          OrderStatus = "active"
	OrderStatusCancelled       OrderStatus = "cancelled"
)

type AccessKey struct {
	SlotNumber       int       `json:"slot_number"`
	Label            string    `json:"label"`
	DeviceID         string    `json:"device_id,omitempty"`
	ClientID         string    `json:"client_id,omitempty"`
	ClientUUID       string    `json:"client_uuid,omitempty"`
	SubscriptionURL  string    `json:"subscription_url,omitempty"`
	SubscriptionText string    `json:"subscription_text,omitempty"`
	PrimaryVLESSURL  string    `json:"primary_vless_url,omitempty"`
	VLESSURLs        []string  `json:"vless_urls,omitempty"`
	ProfileYAMLs     []string  `json:"profile_yamls,omitempty"`
	IssuedAt         time.Time `json:"issued_at"`
}

type PromoCode struct {
	Code            string            `json:"code"`
	Name            string            `json:"name"`
	DiscountPercent int               `json:"discount_percent"`
	MaxUses         int               `json:"max_uses,omitempty"`
	UsedCount       int               `json:"used_count,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	Active          bool              `json:"active"`
	Redemptions     []PromoRedemption `json:"redemptions,omitempty"`
}

type PromoRedemption struct {
	OrderID    string    `json:"order_id"`
	AccountID  string    `json:"account_id,omitempty"`
	UsedAt     time.Time `json:"used_at"`
	OrderTotal int64     `json:"order_total"`
}

type PromoCreateRequest struct {
	Code            string
	Name            string
	DiscountPercent int
	MaxUses         int
	ExpiresAfter    time.Duration
}

type Order struct {
	ID                    string       `json:"id"`
	AccountID             string       `json:"account_id,omitempty"`
	AccountToken          string       `json:"account_token,omitempty"`
	AccessToken           string       `json:"access_token"`
	Status                OrderStatus  `json:"status"`
	ProvisionError        string       `json:"provision_error,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
	ActivatedAt           *time.Time   `json:"activated_at,omitempty"`
	PlanID                string       `json:"plan_id"`
	PlanName              string       `json:"plan_name"`
	PlanBadge             string       `json:"plan_badge,omitempty"`
	PlanDescription       string       `json:"plan_description,omitempty"`
	PriceMinor            int64        `json:"price_minor"`
	Currency              string       `json:"currency"`
	DurationDays          int          `json:"duration_days,omitempty"`
	DeliveryMode          DeliveryMode `json:"delivery_mode"`
	CustomerName          string       `json:"customer_name"`
	Contact               string       `json:"contact"`
	DeviceLabel           string       `json:"device_label,omitempty"`
	DeviceCount           int          `json:"device_count,omitempty"`
	Months                int          `json:"months,omitempty"`
	DiscountPercent       int          `json:"discount_percent,omitempty"`
	BaseMonthlyPriceMinor int64        `json:"base_monthly_price_minor,omitempty"`
	SubtotalMinor         int64        `json:"subtotal_minor,omitempty"`
	DiscountMinor         int64        `json:"discount_minor,omitempty"`
	PromoCode             string       `json:"promo_code,omitempty"`
	PromoName             string       `json:"promo_name,omitempty"`
	PromoDiscountPercent  int          `json:"promo_discount_percent,omitempty"`
	PromoDiscountMinor    int64        `json:"promo_discount_minor,omitempty"`
	Note                  string       `json:"note,omitempty"`
	InviteCode            string       `json:"invite_code,omitempty"`
	InviteRedeemURL       string       `json:"invite_redeem_url,omitempty"`
	InviteAPIURL          string       `json:"invite_api_url,omitempty"`
	PublicAPI             string       `json:"public_api,omitempty"`
	ClientID              string       `json:"client_id,omitempty"`
	ClientUUID            string       `json:"client_uuid,omitempty"`
	SubscriptionURL       string       `json:"subscription_url,omitempty"`
	SubscriptionText      string       `json:"subscription_text,omitempty"`
	PrimaryVLESSURL       string       `json:"primary_vless_url,omitempty"`
	VLESSURLs             []string     `json:"vless_urls,omitempty"`
	ProfileYAMLs          []string     `json:"profile_yamls,omitempty"`
	AccessKeys            []AccessKey  `json:"access_keys,omitempty"`
}

type orderSnapshot struct {
	Version   int         `json:"version"`
	UpdatedAt time.Time   `json:"updated_at"`
	Orders    []Order     `json:"orders,omitempty"`
	Promos    []PromoCode `json:"promos,omitempty"`
}

type OrderStore struct {
	path          string
	screenshotDir string
	mu            sync.Mutex
}

func NewOrderStore(path string) *OrderStore {
	baseDir := filepath.Dir(path)
	return &OrderStore{
		path:          path,
		screenshotDir: filepath.Join(baseDir, "screenshots"),
	}
}

func (s *OrderStore) ScreenshotDir() string {
	return s.screenshotDir
}

func (s *OrderStore) List() ([]Order, error) {
	snapshot, err := s.load()
	if err != nil {
		return nil, err
	}
	orders := append([]Order(nil), snapshot.Orders...)
	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
	return orders, nil
}

func (s *OrderStore) ListPromos() ([]PromoCode, error) {
	snapshot, err := s.load()
	if err != nil {
		return nil, err
	}
	promos := append([]PromoCode(nil), snapshot.Promos...)
	sort.SliceStable(promos, func(i, j int) bool {
		return promos[i].CreatedAt.After(promos[j].CreatedAt)
	})
	return promos, nil
}

func (s *OrderStore) Find(orderID string) (Order, error) {
	snapshot, err := s.load()
	if err != nil {
		return Order{}, err
	}
	for _, order := range snapshot.Orders {
		if order.ID == orderID {
			return order, nil
		}
	}
	return Order{}, errors.New("order not found")
}

func (s *OrderStore) FindPromo(code string) (PromoCode, error) {
	snapshot, err := s.load()
	if err != nil {
		return PromoCode{}, err
	}
	promo := snapshot.findPromo(code)
	if promo == nil {
		return PromoCode{}, errors.New("промокод не найден")
	}
	return *promo, nil
}

func (s *OrderStore) FindByAccount(accountID string) ([]Order, error) {
	snapshot, err := s.load()
	if err != nil {
		return nil, err
	}
	normalizedAccountID := strings.TrimSpace(accountID)
	if normalizedAccountID == "" {
		return nil, errors.New("account not found")
	}
	orders := make([]Order, 0)
	for _, order := range snapshot.Orders {
		if strings.TrimSpace(order.AccountID) == normalizedAccountID {
			orders = append(orders, order)
		}
	}
	if len(orders) == 0 {
		return nil, errors.New("account not found")
	}
	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
	return orders, nil
}

func (s *OrderStore) FindByAccountToken(accountToken string) (string, []Order, error) {
	snapshot, err := s.load()
	if err != nil {
		return "", nil, err
	}
	normalizedToken := strings.TrimSpace(accountToken)
	if normalizedToken == "" {
		return "", nil, errors.New("account token not found")
	}
	orders := make([]Order, 0)
	accountID := ""
	for _, order := range snapshot.Orders {
		if strings.TrimSpace(order.AccountToken) != normalizedToken {
			continue
		}
		if accountID == "" {
			accountID = strings.TrimSpace(order.AccountID)
		}
		orders = append(orders, order)
	}
	if accountID == "" || len(orders) == 0 {
		return "", nil, errors.New("account token not found")
	}
	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
	return accountID, orders, nil
}

func (s *OrderStore) Create(order Order) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return Order{}, err
	}
	if strings.TrimSpace(order.PromoCode) != "" {
		promo := snapshot.findPromo(order.PromoCode)
		if promo == nil {
			return Order{}, errors.New("промокод не найден")
		}
		now := time.Now().UTC()
		if !promo.isRedeemable(now) {
			return Order{}, promo.redeemError(now)
		}
		promo.Redemptions = append(promo.Redemptions, PromoRedemption{
			OrderID:    order.ID,
			AccountID:  order.AccountID,
			UsedAt:     now,
			OrderTotal: order.PriceMinor,
		})
		promo.UsedCount = len(promo.Redemptions)
		promo.Active = promo.isRedeemable(now)
	}
	snapshot.Version++
	snapshot.UpdatedAt = time.Now().UTC()
	snapshot.Orders = append(snapshot.Orders, order)
	if err := s.saveLocked(snapshot); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *OrderStore) Update(orderID string, mutator func(*Order) error) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return Order{}, err
	}

	for index := range snapshot.Orders {
		if snapshot.Orders[index].ID != orderID {
			continue
		}
		if err := mutator(&snapshot.Orders[index]); err != nil {
			return Order{}, err
		}
		snapshot.Orders[index].UpdatedAt = time.Now().UTC()
		snapshot.Version++
		snapshot.UpdatedAt = time.Now().UTC()
		if err := s.saveLocked(snapshot); err != nil {
			return Order{}, err
		}
		return snapshot.Orders[index], nil
	}

	return Order{}, errors.New("order not found")
}

func (s *OrderStore) CreatePromo(input PromoCreateRequest) (PromoCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return PromoCode{}, err
	}
	code, err := normalizeCustomPromoCode(input.Code)
	if err != nil {
		return PromoCode{}, err
	}
	if snapshot.findPromo(code) != nil {
		return PromoCode{}, errors.New("промокод уже существует")
	}
	if strings.TrimSpace(input.Name) == "" {
		return PromoCode{}, errors.New("название промокода обязательно")
	}
	if input.DiscountPercent < 0 || input.DiscountPercent > 100 {
		return PromoCode{}, errors.New("скидка по промокоду должна быть от 0 до 100")
	}
	if input.MaxUses < 0 {
		return PromoCode{}, errors.New("лимит использований не может быть отрицательным")
	}
	now := time.Now().UTC()
	promo := PromoCode{
		Code:            code,
		Name:            strings.TrimSpace(input.Name),
		DiscountPercent: input.DiscountPercent,
		MaxUses:         input.MaxUses,
		CreatedAt:       now,
		Active:          true,
	}
	if input.ExpiresAfter > 0 {
		expiresAt := now.Add(input.ExpiresAfter)
		promo.ExpiresAt = &expiresAt
	}
	snapshot.Promos = append(snapshot.Promos, promo)
	snapshot.Version++
	snapshot.UpdatedAt = now
	if err := s.saveLocked(snapshot); err != nil {
		return PromoCode{}, err
	}
	return promo, nil
}

func (s *OrderStore) load() (orderSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *OrderStore) loadLocked() (orderSnapshot, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return orderSnapshot{}, nil
		}
		return orderSnapshot{}, fmt.Errorf("read orders: %w", err)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return orderSnapshot{}, nil
	}

	var snapshot orderSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return orderSnapshot{}, fmt.Errorf("decode orders: %w", err)
	}
	snapshot.normalize()
	return snapshot, nil
}

func (s *OrderStore) saveLocked(snapshot orderSnapshot) error {
	snapshot.normalize()
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode orders: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create orders directory: %w", err)
	}
	if err := os.MkdirAll(s.screenshotDir, 0o755); err != nil {
		return fmt.Errorf("create screenshots directory: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return fmt.Errorf("write orders: %w", err)
	}
	return nil
}

func (s *orderSnapshot) normalize() {
	now := time.Now().UTC()
	for index := range s.Promos {
		s.Promos[index].Code = strings.TrimSpace(strings.ToLower(s.Promos[index].Code))
		s.Promos[index].Name = strings.TrimSpace(s.Promos[index].Name)
		if s.Promos[index].DiscountPercent < 0 {
			s.Promos[index].DiscountPercent = 0
		}
		if s.Promos[index].DiscountPercent > 100 {
			s.Promos[index].DiscountPercent = 100
		}
		if s.Promos[index].MaxUses < 0 {
			s.Promos[index].MaxUses = 0
		}
		if s.Promos[index].UsedCount < len(s.Promos[index].Redemptions) {
			s.Promos[index].UsedCount = len(s.Promos[index].Redemptions)
		}
		s.Promos[index].Active = s.Promos[index].isRedeemable(now)
		sort.SliceStable(s.Promos[index].Redemptions, func(i, j int) bool {
			return s.Promos[index].Redemptions[i].UsedAt.Before(s.Promos[index].Redemptions[j].UsedAt)
		})
	}
}

func newOrderID() (string, error) {
	return randomToken("ord", 6)
}

func newAccountID() (string, error) {
	return randomToken("acct", 6)
}

func newAccountToken() (string, error) {
	return randomToken("site", 18)
}

func newAccessToken() (string, error) {
	return randomToken("", 16)
}

func randomToken(prefix string, byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if prefix == "" {
		return token, nil
	}
	return prefix + "-" + token, nil
}

func (s *orderSnapshot) findPromo(code string) *PromoCode {
	needle := strings.TrimSpace(strings.ToLower(code))
	if needle == "" {
		return nil
	}
	for index := range s.Promos {
		if strings.TrimSpace(strings.ToLower(s.Promos[index].Code)) == needle {
			return &s.Promos[index]
		}
	}
	return nil
}

func normalizeCustomPromoCode(raw string) (string, error) {
	code := strings.TrimSpace(strings.ToLower(raw))
	if code == "" {
		return "", errors.New("промокод пустой")
	}
	if len(code) < 4 || len(code) > 64 {
		return "", errors.New("длина промокода должна быть от 4 до 64 символов")
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return "", errors.New("промокод может содержать только символы [a-z0-9-_.]")
		}
	}
	return code, nil
}

func (p PromoCode) isRedeemable(now time.Time) bool {
	if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
		return false
	}
	if p.MaxUses > 0 && len(p.Redemptions) >= p.MaxUses {
		return false
	}
	return true
}

func (p PromoCode) redeemError(now time.Time) error {
	switch {
	case p.ExpiresAt != nil && now.After(*p.ExpiresAt):
		return errors.New("срок действия промокода истек")
	case p.MaxUses > 0 && len(p.Redemptions) >= p.MaxUses:
		return errors.New("достигнут лимит использований промокода")
	default:
		return errors.New("промокод неактивен")
	}
}
