package reality

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"novpn/internal/config"
)

type RegistryStore struct {
	path   string
	logger *slog.Logger
	mu     sync.Mutex
}

type Registry struct {
	Version            int            `json:"version"`
	UpdatedAt          time.Time      `json:"updated_at"`
	Server             RegistryServer `json:"server"`
	BootstrapClientID  string         `json:"bootstrap_client_id,omitempty"`
	Invites           []InviteRecord  `json:"invites,omitempty"`
	Clients           []ClientRecord  `json:"clients,omitempty"`
}

type RegistryServer struct {
	PublicHost  string `json:"public_host"`
	PublicPort  int    `json:"public_port"`
	Target      string `json:"target"`
	ServerName  string `json:"server_name"`
	Fingerprint string `json:"fingerprint"`
	Flow        string `json:"flow"`
}

type InviteRecord struct {
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Note               string     `json:"note,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	RedeemedAt         *time.Time `json:"redeemed_at,omitempty"`
	RedeemedClientID   string     `json:"redeemed_client_id,omitempty"`
	RedeemedDeviceID   string     `json:"redeemed_device_id,omitempty"`
	RedeemedDeviceName string     `json:"redeemed_device_name,omitempty"`
	Active             bool       `json:"active"`
}

type ClientRecord struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	DeviceID    string     `json:"device_id"`
	DeviceName  string     `json:"device_name"`
	UUID        string     `json:"uuid"`
	Email       string     `json:"email"`
	InviteCode  string     `json:"invite_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	Active      bool       `json:"active"`
}

type RegistrySummary struct {
	Server           RegistryServer `json:"server"`
	BootstrapClientID string         `json:"bootstrap_client_id,omitempty"`
	TotalClients     int            `json:"total_clients"`
	ActiveClients    int            `json:"active_clients"`
	RevokedClients   int            `json:"revoked_clients"`
	TotalInvites     int            `json:"total_invites"`
	PendingInvites   int            `json:"pending_invites"`
	RedeemedInvites  int            `json:"redeemed_invites"`
}

type InviteCreateRequest struct {
	Name        string
	Note        string
	ExpiresAfter time.Duration
}

type InviteRedeemResult struct {
	Invite InviteRecord
	Client ClientRecord
}

func NewRegistryStore(path string, logger *slog.Logger) *RegistryStore {
	return &RegistryStore{
		path:   path,
		logger: logger,
	}
}

func (s *RegistryStore) Load() (Registry, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Registry{}, nil
		}
		return Registry{}, fmt.Errorf("read registry: %w", err)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return Registry{}, nil
	}

	var registry Registry
	if err := json.Unmarshal(payload, &registry); err != nil {
		return Registry{}, fmt.Errorf("unmarshal registry: %w", err)
	}

	registry.normalize()
	return registry, nil
}

func (s *RegistryStore) Update(mutator func(*Registry) error) (Registry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	registry, err := s.Load()
	if err != nil {
		return Registry{}, err
	}

	if err := mutator(&registry); err != nil {
		return Registry{}, err
	}

	registry.normalize()
	payload, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return Registry{}, fmt.Errorf("marshal registry: %w", err)
	}
	payload = append(payload, '\n')

	if err := writeFileAtomically(s.path, payload, 0o600); err != nil {
		return Registry{}, fmt.Errorf("write registry: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("registry updated", "path", s.path, "clients", len(registry.Clients), "invites", len(registry.Invites))
	}

	return registry, nil
}

func (s *RegistryStore) EnsureBootstrap(state State, cfg config.RealityConfig) (Registry, error) {
	return s.Update(func(registry *Registry) error {
		registry.Server = snapshotServer(cfg)
		if registry.BootstrapClientID == "" {
			registry.BootstrapClientID = "bootstrap"
		}

		now := time.Now().UTC()
		bootstrap := registry.findClient(registry.BootstrapClientID)
		if bootstrap == nil {
			registry.Clients = append(registry.Clients, ClientRecord{
				ID:         registry.BootstrapClientID,
				Name:       "Bootstrap device",
				DeviceID:   "bootstrap",
				DeviceName: "Bootstrap device",
				UUID:       state.UUID,
				Email:      "bootstrap@novpn",
				InviteCode: "bootstrap",
				CreatedAt:  now,
				UpdatedAt:  now,
				Active:     true,
			})
			return nil
		}

		bootstrap.UUID = state.UUID
		bootstrap.Active = true
		bootstrap.UpdatedAt = now
		bootstrap.RevokedAt = nil
		bootstrap.DeviceName = bootstrapDeviceName(bootstrap.DeviceName)
		return nil
	})
}

func (s *RegistryStore) Summary(cfg config.RealityConfig) (RegistrySummary, error) {
	registry, err := s.Load()
	if err != nil {
		return RegistrySummary{}, err
	}

	summary := RegistrySummary{
		Server:           snapshotServer(cfg),
		BootstrapClientID: registry.BootstrapClientID,
		TotalClients:     len(registry.Clients),
		TotalInvites:     len(registry.Invites),
	}
	for _, client := range registry.Clients {
		if client.Active && client.RevokedAt == nil {
			summary.ActiveClients++
		} else {
			summary.RevokedClients++
		}
	}
	for _, invite := range registry.Invites {
		if invite.RedeemedAt == nil && invite.Active {
			summary.PendingInvites++
		}
		if invite.RedeemedAt != nil {
			summary.RedeemedInvites++
		}
	}

	return summary, nil
}

func (s *RegistryStore) ListClients() ([]ClientRecord, error) {
	registry, err := s.Load()
	if err != nil {
		return nil, err
	}
	clients := append([]ClientRecord(nil), registry.Clients...)
	sort.SliceStable(clients, func(i, j int) bool {
		return clients[i].CreatedAt.Before(clients[j].CreatedAt)
	})
	return clients, nil
}

func (s *RegistryStore) ListInvites() ([]InviteRecord, error) {
	registry, err := s.Load()
	if err != nil {
		return nil, err
	}
	invites := append([]InviteRecord(nil), registry.Invites...)
	sort.SliceStable(invites, func(i, j int) bool {
		return invites[i].CreatedAt.Before(invites[j].CreatedAt)
	})
	return invites, nil
}

func (s *RegistryStore) CreateInvite(input InviteCreateRequest) (InviteRecord, error) {
	var created InviteRecord
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		created = InviteRecord{
			Code:      generateRegistryToken("inv"),
			Name:      firstNonEmpty(strings.TrimSpace(input.Name), "New device"),
			Note:      strings.TrimSpace(input.Note),
			CreatedAt: now,
			Active:    true,
		}
		if input.ExpiresAfter > 0 {
			expiresAt := now.Add(input.ExpiresAfter)
			created.ExpiresAt = &expiresAt
		}
		registry.Invites = append(registry.Invites, created)
		return nil
	})
	return created, err
}

func (s *RegistryStore) RedeemInvite(code string, deviceID string, deviceName string) (InviteRedeemResult, error) {
	var result InviteRedeemResult
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		invite := registry.findInvite(code)
		if invite == nil {
			return errors.New("invite not found")
		}
		if !invite.Active || invite.RedeemedAt != nil {
			return errors.New("invite already redeemed")
		}
		if invite.ExpiresAt != nil && now.After(*invite.ExpiresAt) {
			invite.Active = false
			return errors.New("invite expired")
		}

		normalizedDeviceID := strings.TrimSpace(deviceID)
		if normalizedDeviceID != "" {
			for _, existing := range registry.Clients {
				if existing.DeviceID == normalizedDeviceID && existing.Active && existing.RevokedAt == nil {
					return fmt.Errorf("device %q already has an active profile", normalizedDeviceID)
				}
			}
		} else {
			normalizedDeviceID = generateRegistryToken("device")
		}

		clientUUID, err := generateUUID()
		if err != nil {
			return err
		}

		client := ClientRecord{
			ID:         generateRegistryToken("client"),
			Name:       firstNonEmpty(strings.TrimSpace(invite.Name), "Imported device"),
			DeviceID:   normalizedDeviceID,
			DeviceName: firstNonEmpty(strings.TrimSpace(deviceName), "Imported device"),
			UUID:       clientUUID,
			Email:      slugEmail(invite.Name, deviceName),
			InviteCode: invite.Code,
			CreatedAt:  now,
			UpdatedAt:  now,
			Active:     true,
		}

		invite.Active = false
		invite.RedeemedAt = &now
		invite.RedeemedClientID = client.ID
		invite.RedeemedDeviceID = client.DeviceID
		invite.RedeemedDeviceName = client.DeviceName

		registry.Clients = append(registry.Clients, client)
		result = InviteRedeemResult{
			Invite: *invite,
			Client: client,
		}
		return nil
	})
	return result, err
}

func (s *RegistryStore) RevokeClient(clientID string) (ClientRecord, error) {
	var updated ClientRecord
	_, err := s.Update(func(registry *Registry) error {
		client := registry.findClient(clientID)
		if client == nil {
			return errors.New("client not found")
		}
		now := time.Now().UTC()
		client.Active = false
		client.RevokedAt = &now
		client.UpdatedAt = now
		updated = *client
		return nil
	})
	return updated, err
}

func (s *RegistryStore) PrimaryClient() (ClientRecord, error) {
	registry, err := s.Load()
	if err != nil {
		return ClientRecord{}, err
	}
	for _, client := range registry.Clients {
		if client.Active && client.RevokedAt == nil {
			return client, nil
		}
	}
	return ClientRecord{}, errors.New("no active clients in registry")
}

func (r Registry) ActiveClients() []ClientRecord {
	clients := make([]ClientRecord, 0, len(r.Clients))
	for _, client := range r.Clients {
		if client.Active && client.RevokedAt == nil {
			clients = append(clients, client)
		}
	}
	sort.SliceStable(clients, func(i, j int) bool {
		return clients[i].CreatedAt.Before(clients[j].CreatedAt)
	})
	return clients
}

func (r Registry) ActiveXrayClients(flow string) []any {
	clients := r.ActiveClients()
	entries := make([]any, 0, len(clients))
	for _, client := range clients {
		entries = append(entries, map[string]any{
			"id":    client.UUID,
			"flow":  flow,
			"email": client.Email,
		})
	}
	return entries
}

func (r Registry) ActiveClientProfile(state State, cfg config.RealityConfig) (ClientProfile, error) {
	client, ok := r.primaryActiveClient()
	if !ok {
		return ClientProfile{}, errors.New("no active client available")
	}
	return buildClientProfileFor(cfg, state, client), nil
}

func (r Registry) primaryActiveClient() (ClientRecord, bool) {
	for _, client := range r.Clients {
		if client.Active && client.RevokedAt == nil {
			return client, true
		}
	}
	return ClientRecord{}, false
}

func (r *Registry) normalize() {
	if r.Version == 0 {
		r.Version = 1
	}
	sort.SliceStable(r.Invites, func(i, j int) bool {
		return r.Invites[i].CreatedAt.Before(r.Invites[j].CreatedAt)
	})
	sort.SliceStable(r.Clients, func(i, j int) bool {
		return r.Clients[i].CreatedAt.Before(r.Clients[j].CreatedAt)
	})
}

func (r *Registry) findInvite(code string) *InviteRecord {
	for i := range r.Invites {
		if r.Invites[i].Code == code {
			return &r.Invites[i]
		}
	}
	return nil
}

func (r *Registry) findClient(id string) *ClientRecord {
	for i := range r.Clients {
		if r.Clients[i].ID == id {
			return &r.Clients[i]
		}
	}
	return nil
}

func snapshotServer(cfg config.RealityConfig) RegistryServer {
	serverName := ""
	if len(cfg.ServerNames) > 0 {
		serverName = cfg.ServerNames[0]
	}

	return RegistryServer{
		PublicHost:  cfg.PublicHost,
		PublicPort:  cfg.PublicPort,
		Target:      cfg.Target,
		ServerName:  serverName,
		Fingerprint: cfg.Fingerprint,
		Flow:        cfg.Flow,
	}
}

func buildClientProfileFor(cfg config.RealityConfig, state State, client ClientRecord) ClientProfile {
	shortID := ""
	if len(state.ShortIDs) > 0 {
		shortID = state.ShortIDs[0]
	}

	serverName := ""
	if len(cfg.ServerNames) > 0 {
		serverName = cfg.ServerNames[0]
	}

	name := client.Name
	if strings.TrimSpace(name) == "" {
		name = client.DeviceName
	}
	if strings.TrimSpace(name) == "" {
		name = "novpn-device"
	}

	return ClientProfile{
		GeneratedAt: time.Now().UTC(),
		Name:        name,
		Type:        "vless-reality",
		Address:     cfg.PublicHost,
		Port:        cfg.PublicPort,
		UUID:        client.UUID,
		Flow:        cfg.Flow,
		Network:     "tcp",
		Security:    "reality",
		ServerName:  serverName,
		Fingerprint: cfg.Fingerprint,
		PublicKey:   state.PublicKey,
		ShortID:     shortID,
		ShortIDs:    append([]string(nil), state.ShortIDs...),
		SpiderX:     cfg.SpiderX,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func bootstrapDeviceName(value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return "Bootstrap device"
}

func slugEmail(parts ...string) string {
	combined := strings.ToLower(strings.Join(parts, "-"))
	combined = strings.TrimSpace(combined)
	if combined == "" {
		return "client@novpn"
	}
	builder := strings.Builder{}
	for _, r := range combined {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	email := strings.Trim(builder.String(), "-")
	if email == "" {
		email = "client"
	}
	return email + "@novpn"
}

func generateRegistryToken(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405")
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}
