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
	DeliveryModeInviteCode   DeliveryMode = "invite_code"
	DeliveryModeProfileBundle DeliveryMode = "profile_bundle"
)

type OrderStatus string

const (
	OrderStatusPendingReview OrderStatus = "pending_review"
	OrderStatusConfirmed     OrderStatus = "confirmed"
	OrderStatusRejected      OrderStatus = "rejected"
)

type Order struct {
	ID                     string       `json:"id"`
	AccessToken            string       `json:"access_token"`
	Status                 OrderStatus  `json:"status"`
	ReviewReason           string       `json:"review_reason,omitempty"`
	ReviewedBy             string       `json:"reviewed_by,omitempty"`
	CreatedAt              time.Time    `json:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at"`
	ReviewedAt             *time.Time   `json:"reviewed_at,omitempty"`
	PlanID                 string       `json:"plan_id"`
	PlanName               string       `json:"plan_name"`
	PlanBadge              string       `json:"plan_badge,omitempty"`
	PlanDescription        string       `json:"plan_description,omitempty"`
	PriceMinor             int64        `json:"price_minor"`
	Currency               string       `json:"currency"`
	DurationDays           int          `json:"duration_days,omitempty"`
	DeliveryMode           DeliveryMode `json:"delivery_mode"`
	CustomerName           string       `json:"customer_name"`
	Contact                string       `json:"contact"`
	DeviceLabel            string       `json:"device_label,omitempty"`
	Note                   string       `json:"note,omitempty"`
	ScreenshotFile         string       `json:"screenshot_file"`
	ScreenshotOriginalName string       `json:"screenshot_original_name,omitempty"`
	ScreenshotContentType  string       `json:"screenshot_content_type,omitempty"`
	InviteCode             string       `json:"invite_code,omitempty"`
	InviteRedeemURL        string       `json:"invite_redeem_url,omitempty"`
	InviteAPIURL           string       `json:"invite_api_url,omitempty"`
	PublicAPI              string       `json:"public_api,omitempty"`
	ClientID               string       `json:"client_id,omitempty"`
	ClientUUID             string       `json:"client_uuid,omitempty"`
	SubscriptionURL        string       `json:"subscription_url,omitempty"`
	SubscriptionText       string       `json:"subscription_text,omitempty"`
	PrimaryVLESSURL        string       `json:"primary_vless_url,omitempty"`
	VLESSURLs              []string     `json:"vless_urls,omitempty"`
	ProfileYAMLs           []string     `json:"profile_yamls,omitempty"`
}

type orderSnapshot struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Orders    []Order   `json:"orders,omitempty"`
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

func (s *OrderStore) Create(order Order) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return Order{}, err
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
	return snapshot, nil
}

func (s *OrderStore) saveLocked(snapshot orderSnapshot) error {
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

func newOrderID() (string, error) {
	return randomToken("ord", 6)
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
