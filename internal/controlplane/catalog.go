package controlplane

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"novpn/internal/config"
)

type CatalogStore struct {
	path   string
	logger *slog.Logger
	mu     sync.Mutex
}

type CatalogSnapshot struct {
	Version   int                `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
	Servers   []ServerNode       `json:"servers,omitempty"`
	Plans     []SubscriptionPlan `json:"plans,omitempty"`
}

type ServerNode struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	Flow          string    `json:"flow,omitempty"`
	ServerName    string    `json:"server_name"`
	Fingerprint   string    `json:"fingerprint"`
	PublicKey     string    `json:"public_key,omitempty"`
	ShortID       string    `json:"short_id,omitempty"`
	ShortIDs      []string  `json:"short_ids,omitempty"`
	SpiderX       string    `json:"spider_x,omitempty"`
	LocationLabel string    `json:"location_label,omitempty"`
	VPNOnly       bool      `json:"vpn_only,omitempty"`
	Active        bool      `json:"active"`
	Primary       bool      `json:"primary,omitempty"`
	SortOrder     int       `json:"sort_order,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SubscriptionPlan struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description,omitempty"`
	DurationDays      int       `json:"duration_days,omitempty"`
	TrafficLimitBytes int64     `json:"traffic_limit_bytes,omitempty"`
	PriceMinor        int64     `json:"price_minor,omitempty"`
	Currency          string    `json:"currency,omitempty"`
	ServerIDs         []string  `json:"server_ids,omitempty"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ServerCreateRequest struct {
	ID            string
	Name          string
	Address       string
	Port          int
	Flow          string
	ServerName    string
	Fingerprint   string
	PublicKey     string
	ShortID       string
	ShortIDs      []string
	SpiderX       string
	LocationLabel string
	VPNOnly       bool
	Primary       bool
}

type PlanCreateRequest struct {
	ID                string
	Name              string
	Description       string
	DurationDays      int
	TrafficLimitBytes int64
	PriceMinor        int64
	Currency          string
	ServerIDs         []string
}

func NewCatalogStore(path string, logger *slog.Logger) *CatalogStore {
	return &CatalogStore{
		path:   path,
		logger: logger,
	}
}

func (s *CatalogStore) Load() (CatalogSnapshot, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return CatalogSnapshot{}, nil
		}
		return CatalogSnapshot{}, fmt.Errorf("read catalog: %w", err)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return CatalogSnapshot{}, nil
	}

	var snapshot CatalogSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return CatalogSnapshot{}, fmt.Errorf("unmarshal catalog: %w", err)
	}
	snapshot.normalize()
	return snapshot, nil
}

func (s *CatalogStore) Update(mutator func(*CatalogSnapshot) error) (CatalogSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.Load()
	if err != nil {
		return CatalogSnapshot{}, err
	}
	if err := mutator(&snapshot); err != nil {
		return CatalogSnapshot{}, err
	}
	snapshot.normalize()
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return CatalogSnapshot{}, fmt.Errorf("marshal catalog: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return CatalogSnapshot{}, fmt.Errorf("create catalog directory: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return CatalogSnapshot{}, fmt.Errorf("write catalog: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("catalog updated", "path", s.path, "servers", len(snapshot.Servers), "plans", len(snapshot.Plans))
	}
	return snapshot, nil
}

func (s *CatalogStore) EnsureDefaults(cfg config.RealityConfig) (CatalogSnapshot, error) {
	return s.Update(func(snapshot *CatalogSnapshot) error {
		if snapshot.findServer("primary") == nil {
			serverName := ""
			if len(cfg.ServerNames) > 0 {
				serverName = strings.TrimSpace(cfg.ServerNames[0])
			}
			snapshot.Servers = append(snapshot.Servers, ServerNode{
				ID:            "primary",
				Name:          firstNonEmpty(strings.TrimSpace(cfg.PublicHost), "Primary VPN"),
				Address:       strings.TrimSpace(cfg.PublicHost),
				Port:          cfg.PublicPort,
				Flow:          strings.TrimSpace(cfg.Flow),
				ServerName:    serverName,
				Fingerprint:   firstNonEmpty(strings.TrimSpace(cfg.Fingerprint), "chrome"),
				ShortIDs:      append([]string(nil), cfg.ShortIDs...),
				SpiderX:       firstNonEmpty(strings.TrimSpace(cfg.SpiderX), "/"),
				LocationLabel: strings.TrimSpace(cfg.PublicHost),
				Active:        true,
				Primary:       true,
				SortOrder:     10,
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
			})
		}

		sortOrder := nextSort(snapshot.Servers)
		for index, additional := range cfg.AdditionalServers {
			id := buildServerID(additional.Name, additional.PublicHost, index+2)
			if snapshot.findServer(id) != nil {
				continue
			}
			serverName := ""
			if len(additional.ServerNames) > 0 {
				serverName = strings.TrimSpace(additional.ServerNames[0])
			}
			shortIDs := append([]string(nil), additional.ShortIDs...)
			if len(shortIDs) == 0 && strings.TrimSpace(additional.ShortID) != "" {
				shortIDs = append(shortIDs, strings.TrimSpace(additional.ShortID))
			}
			snapshot.Servers = append(snapshot.Servers, ServerNode{
				ID:            id,
				Name:          firstNonEmpty(strings.TrimSpace(additional.Name), fmt.Sprintf("VPN node %d", index+2)),
				Address:       strings.TrimSpace(additional.PublicHost),
				Port:          additional.PublicPort,
				Flow:          firstNonEmpty(strings.TrimSpace(additional.Flow), strings.TrimSpace(cfg.Flow)),
				ServerName:    serverName,
				Fingerprint:   firstNonEmpty(strings.TrimSpace(additional.Fingerprint), strings.TrimSpace(cfg.Fingerprint), "chrome"),
				PublicKey:     strings.TrimSpace(additional.PublicKey),
				ShortID:       strings.TrimSpace(additional.ShortID),
				ShortIDs:      shortIDs,
				SpiderX:       firstNonEmpty(strings.TrimSpace(additional.SpiderX), strings.TrimSpace(cfg.SpiderX), "/"),
				LocationLabel: strings.TrimSpace(additional.PublicHost),
				VPNOnly:       additional.VPNOnly,
				Active:        true,
				SortOrder:     sortOrder,
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
			})
			sortOrder += 10
		}
		return nil
	})
}

func (s *CatalogStore) ListServers() ([]ServerNode, error) {
	snapshot, err := s.Load()
	if err != nil {
		return nil, err
	}
	return append([]ServerNode(nil), snapshot.Servers...), nil
}

func (s *CatalogStore) ListPlans() ([]SubscriptionPlan, error) {
	snapshot, err := s.Load()
	if err != nil {
		return nil, err
	}
	return append([]SubscriptionPlan(nil), snapshot.Plans...), nil
}

func (s *CatalogStore) ActivePlans() ([]SubscriptionPlan, error) {
	plans, err := s.ListPlans()
	if err != nil {
		return nil, err
	}
	result := make([]SubscriptionPlan, 0, len(plans))
	for _, plan := range plans {
		if plan.Active {
			result = append(result, plan)
		}
	}
	return result, nil
}

func (s *CatalogStore) FindPlan(planID string) (SubscriptionPlan, error) {
	snapshot, err := s.Load()
	if err != nil {
		return SubscriptionPlan{}, err
	}
	plan := snapshot.findPlan(planID)
	if plan == nil {
		return SubscriptionPlan{}, fmt.Errorf("subscription plan %q not found", strings.TrimSpace(planID))
	}
	return *plan, nil
}

func (s *CatalogStore) FindServers(serverIDs []string) ([]ServerNode, error) {
	snapshot, err := s.Load()
	if err != nil {
		return nil, err
	}
	return snapshot.FindServers(serverIDs), nil
}

func (s *CatalogStore) CreateServer(input ServerCreateRequest) (ServerNode, error) {
	var created ServerNode
	_, err := s.Update(func(snapshot *CatalogSnapshot) error {
		id := normalizeCatalogID(input.ID)
		if id == "" {
			id = buildServerID(input.Name, input.Address, len(snapshot.Servers)+1)
		}
		if snapshot.findServer(id) != nil {
			return fmt.Errorf("server %q already exists", id)
		}
		if strings.TrimSpace(input.Address) == "" {
			return fmt.Errorf("address is required")
		}
		if input.Port <= 0 {
			return fmt.Errorf("port must be greater than zero")
		}
		if strings.TrimSpace(input.ServerName) == "" {
			return fmt.Errorf("server_name is required")
		}
		shortIDs := normalizeStringList(input.ShortIDs)
		if len(shortIDs) == 0 && strings.TrimSpace(input.ShortID) != "" {
			shortIDs = append(shortIDs, strings.TrimSpace(input.ShortID))
		}
		now := time.Now().UTC()
		created = ServerNode{
			ID:            id,
			Name:          firstNonEmpty(strings.TrimSpace(input.Name), id),
			Address:       strings.TrimSpace(input.Address),
			Port:          input.Port,
			Flow:          strings.TrimSpace(input.Flow),
			ServerName:    strings.TrimSpace(input.ServerName),
			Fingerprint:   firstNonEmpty(strings.TrimSpace(input.Fingerprint), "chrome"),
			PublicKey:     strings.TrimSpace(input.PublicKey),
			ShortID:       strings.TrimSpace(input.ShortID),
			ShortIDs:      shortIDs,
			SpiderX:       firstNonEmpty(strings.TrimSpace(input.SpiderX), "/"),
			LocationLabel: strings.TrimSpace(input.LocationLabel),
			VPNOnly:       input.VPNOnly,
			Active:        true,
			Primary:       input.Primary,
			SortOrder:     nextSort(snapshot.Servers),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		snapshot.Servers = append(snapshot.Servers, created)
		return nil
	})
	return created, err
}

func (s *CatalogStore) CreatePlan(input PlanCreateRequest) (SubscriptionPlan, error) {
	var created SubscriptionPlan
	_, err := s.Update(func(snapshot *CatalogSnapshot) error {
		id := normalizeCatalogID(input.ID)
		if id == "" {
			id = buildPlanID(input.Name, len(snapshot.Plans)+1)
		}
		if snapshot.findPlan(id) != nil {
			return fmt.Errorf("subscription plan %q already exists", id)
		}
		if strings.TrimSpace(input.Name) == "" {
			return fmt.Errorf("name is required")
		}
		serverIDs := normalizeServerIDs(input.ServerIDs)
		if len(serverIDs) == 0 {
			return fmt.Errorf("at least one server must be assigned to a subscription plan")
		}
		for _, serverID := range serverIDs {
			if snapshot.findServer(serverID) == nil {
				return fmt.Errorf("server %q not found", serverID)
			}
		}
		now := time.Now().UTC()
		created = SubscriptionPlan{
			ID:                id,
			Name:              strings.TrimSpace(input.Name),
			Description:       strings.TrimSpace(input.Description),
			DurationDays:      clampPositiveInt(input.DurationDays),
			TrafficLimitBytes: clampNonNegativeInt64(input.TrafficLimitBytes),
			PriceMinor:        clampNonNegativeInt64(input.PriceMinor),
			Currency:          strings.ToUpper(strings.TrimSpace(input.Currency)),
			ServerIDs:         serverIDs,
			Active:            true,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		snapshot.Plans = append(snapshot.Plans, created)
		return nil
	})
	return created, err
}

func (snapshot *CatalogSnapshot) normalize() {
	if snapshot.Version < 1 {
		snapshot.Version = 1
	}
	now := time.Now().UTC()
	for index := range snapshot.Servers {
		server := &snapshot.Servers[index]
		server.ID = normalizeCatalogID(server.ID)
		if server.ID == "" {
			server.ID = buildServerID(server.Name, server.Address, index+1)
		}
		server.Name = firstNonEmpty(strings.TrimSpace(server.Name), server.ID)
		server.Address = strings.TrimSpace(server.Address)
		server.Flow = strings.TrimSpace(server.Flow)
		server.ServerName = strings.TrimSpace(server.ServerName)
		server.Fingerprint = firstNonEmpty(strings.TrimSpace(server.Fingerprint), "chrome")
		server.PublicKey = strings.TrimSpace(server.PublicKey)
		server.ShortID = strings.TrimSpace(server.ShortID)
		server.ShortIDs = normalizeStringList(server.ShortIDs)
		if len(server.ShortIDs) == 0 && server.ShortID != "" {
			server.ShortIDs = []string{server.ShortID}
		}
		server.SpiderX = firstNonEmpty(strings.TrimSpace(server.SpiderX), "/")
		if server.CreatedAt.IsZero() {
			server.CreatedAt = now
		}
		if server.UpdatedAt.IsZero() {
			server.UpdatedAt = now
		}
	}
	for index := range snapshot.Plans {
		plan := &snapshot.Plans[index]
		plan.ID = normalizeCatalogID(plan.ID)
		if plan.ID == "" {
			plan.ID = buildPlanID(plan.Name, index+1)
		}
		plan.Name = firstNonEmpty(strings.TrimSpace(plan.Name), plan.ID)
		plan.Description = strings.TrimSpace(plan.Description)
		plan.DurationDays = clampPositiveInt(plan.DurationDays)
		plan.TrafficLimitBytes = clampNonNegativeInt64(plan.TrafficLimitBytes)
		plan.PriceMinor = clampNonNegativeInt64(plan.PriceMinor)
		plan.Currency = strings.ToUpper(strings.TrimSpace(plan.Currency))
		plan.ServerIDs = normalizeServerIDs(plan.ServerIDs)
		if plan.CreatedAt.IsZero() {
			plan.CreatedAt = now
		}
		if plan.UpdatedAt.IsZero() {
			plan.UpdatedAt = now
		}
	}
	sort.SliceStable(snapshot.Servers, func(i, j int) bool {
		if snapshot.Servers[i].SortOrder == snapshot.Servers[j].SortOrder {
			return snapshot.Servers[i].Name < snapshot.Servers[j].Name
		}
		return snapshot.Servers[i].SortOrder < snapshot.Servers[j].SortOrder
	})
	sort.SliceStable(snapshot.Plans, func(i, j int) bool {
		return snapshot.Plans[i].CreatedAt.Before(snapshot.Plans[j].CreatedAt)
	})
	snapshot.UpdatedAt = now
}

func (snapshot CatalogSnapshot) FindServers(serverIDs []string) []ServerNode {
	if len(snapshot.Servers) == 0 {
		return nil
	}
	if len(serverIDs) == 0 {
		servers := make([]ServerNode, 0, len(snapshot.Servers))
		for _, server := range snapshot.Servers {
			if server.Active {
				servers = append(servers, server)
			}
		}
		return servers
	}
	allowed := make(map[string]struct{}, len(serverIDs))
	for _, serverID := range serverIDs {
		allowed[normalizeCatalogID(serverID)] = struct{}{}
	}
	servers := make([]ServerNode, 0, len(snapshot.Servers))
	for _, server := range snapshot.Servers {
		if !server.Active {
			continue
		}
		if _, ok := allowed[server.ID]; ok {
			servers = append(servers, server)
		}
	}
	return servers
}

func (snapshot CatalogSnapshot) findServer(id string) *ServerNode {
	needle := normalizeCatalogID(id)
	for index := range snapshot.Servers {
		if snapshot.Servers[index].ID == needle {
			return &snapshot.Servers[index]
		}
	}
	return nil
}

func (snapshot CatalogSnapshot) findPlan(id string) *SubscriptionPlan {
	needle := normalizeCatalogID(id)
	for index := range snapshot.Plans {
		if snapshot.Plans[index].ID == needle {
			return &snapshot.Plans[index]
		}
	}
	return nil
}

func buildServerID(name string, address string, fallbackIndex int) string {
	base := normalizeCatalogID(firstNonEmpty(name, address))
	if base == "" {
		base = fmt.Sprintf("vpn-node-%d", fallbackIndex)
	}
	return base
}

func buildPlanID(name string, fallbackIndex int) string {
	base := normalizeCatalogID(name)
	if base == "" {
		base = fmt.Sprintf("plan-%d", fallbackIndex)
	}
	return base
}

func nextSort(servers []ServerNode) int {
	maxSort := 0
	for _, server := range servers {
		if server.SortOrder > maxSort {
			maxSort = server.SortOrder
		}
	}
	if maxSort == 0 {
		return 10
	}
	return maxSort + 10
}

func normalizeCatalogID(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	builder := strings.Builder{}
	for _, ch := range trimmed {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		default:
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func normalizeServerIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeCatalogID(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func clampPositiveInt(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func clampNonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
