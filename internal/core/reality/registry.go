package reality

import (
	"bytes"
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
	Version           int            `json:"version"`
	UpdatedAt         time.Time      `json:"updated_at"`
	Server            RegistryServer `json:"server"`
	BootstrapClientID string         `json:"bootstrap_client_id,omitempty"`
	Invites           []InviteRecord `json:"invites,omitempty"`
	Promos            []PromoRecord  `json:"promos,omitempty"`
	Clients           []ClientRecord `json:"clients,omitempty"`
	BlockedDevices    []BlockedDeviceRecord `json:"blocked_devices,omitempty"`
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
	PlanID             string     `json:"plan_id,omitempty"`
	PlanName           string     `json:"plan_name,omitempty"`
	AllowedServerIDs   []string   `json:"allowed_server_ids,omitempty"`
	AccessDurationDays int        `json:"access_duration_days,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	MaxUses            int        `json:"max_uses,omitempty"`
	ActiveUses         int        `json:"active_uses,omitempty"`
	RedeemedUses       int        `json:"redeemed_uses,omitempty"`
	TrafficLimitBytes  int64      `json:"traffic_limit_bytes,omitempty"`
	RedeemedAt         *time.Time `json:"redeemed_at,omitempty"`
	RedeemedClientID   string     `json:"redeemed_client_id,omitempty"`
	RedeemedDeviceID   string     `json:"redeemed_device_id,omitempty"`
	RedeemedDeviceName string     `json:"redeemed_device_name,omitempty"`
	Active             bool       `json:"active"`
}

type PromoRecord struct {
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	Note           string            `json:"note,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	BonusBytes     int64             `json:"bonus_bytes"`
	MaxUses        int               `json:"max_uses,omitempty"`
	RedeemedUses   int               `json:"redeemed_uses,omitempty"`
	LastRedeemedAt *time.Time        `json:"last_redeemed_at,omitempty"`
	Redemptions    []PromoRedemption `json:"redemptions,omitempty"`
	Active         bool              `json:"active"`
}

type PromoRedemption struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	ClientID   string    `json:"client_id"`
	RedeemedAt time.Time `json:"redeemed_at"`
	BonusBytes int64     `json:"bonus_bytes"`
}

type PromoActivationMode string

const (
	PromoActivationModeBonus PromoActivationMode = "bonus"
	PromoActivationModeTrial PromoActivationMode = "trial"
)

type ClientRecord struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	DeviceID             string                 `json:"device_id"`
	DeviceName           string                 `json:"device_name"`
	ObservedDevices      []ObservedDeviceRecord `json:"observed_devices,omitempty"`
	UUID                 string                 `json:"uuid"`
	Email                string                 `json:"email"`
	InviteCode           string                 `json:"invite_code,omitempty"`
	PlanID               string                 `json:"plan_id,omitempty"`
	PlanName             string                 `json:"plan_name,omitempty"`
	AllowedServerIDs     []string               `json:"allowed_server_ids,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	RevokedAt            *time.Time             `json:"revoked_at,omitempty"`
	LastSeenAt           *time.Time             `json:"last_seen_at,omitempty"`
	AccessExpiresAt      *time.Time             `json:"access_expires_at,omitempty"`
	TrafficLimitBytes    int64                  `json:"traffic_limit_bytes,omitempty"`
	TrafficBonusBytes    int64                  `json:"traffic_bonus_bytes,omitempty"`
	TrafficUsedBytes     int64                  `json:"traffic_used_bytes,omitempty"`
	TrafficObservedBytes int64                  `json:"traffic_observed_bytes,omitempty"`
	LastTrafficSyncAt    *time.Time             `json:"last_traffic_sync_at,omitempty"`
	TrafficBlockedAt     *time.Time             `json:"traffic_blocked_at,omitempty"`
	Active               bool                   `json:"active"`
}

type ObservedDeviceRecord struct {
	DeviceID        string    `json:"device_id"`
	DeviceName      string    `json:"device_name,omitempty"`
	DeviceOS        string    `json:"device_os,omitempty"`
	DeviceOSVersion string    `json:"device_os_version,omitempty"`
	UserAgent       string    `json:"user_agent,omitempty"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

type SubscriptionDeviceObservation struct {
	DeviceID        string
	DeviceName      string
	DeviceOS        string
	DeviceOSVersion string
	UserAgent       string
	SeenAt          time.Time
}

type BlockedDeviceRecord struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name,omitempty"`
	ClientID   string    `json:"client_id,omitempty"`
	ClientUUID string    `json:"client_uuid,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Source     string    `json:"source,omitempty"`
	BlockedAt  time.Time `json:"blocked_at"`
}

type BlacklistResult struct {
	Client         ClientRecord         `json:"client"`
	BlockedDevices []BlockedDeviceRecord `json:"blocked_devices"`
}

type RegistrySummary struct {
	Server                RegistryServer `json:"server"`
	BootstrapClientID     string         `json:"bootstrap_client_id,omitempty"`
	TotalClients          int            `json:"total_clients"`
	ActiveClients         int            `json:"active_clients"`
	TrafficBlockedClients int            `json:"traffic_blocked_clients"`
	RevokedClients        int            `json:"revoked_clients"`
	TotalInvites          int            `json:"total_invites"`
	PendingInvites        int            `json:"pending_invites"`
	RedeemedInvites       int            `json:"redeemed_invites"`
	TotalPromos           int            `json:"total_promos"`
	ActivePromos          int            `json:"active_promos"`
	TotalTrafficBytes     int64          `json:"total_traffic_bytes"`
}

type InviteCreateRequest struct {
	Name               string
	Note               string
	PlanID             string
	PlanName           string
	AllowedServerIDs   []string
	AccessDurationDays int
	MaxUses            int
	TrafficLimitBytes  int64
	ExpiresAfter       time.Duration
}

type PromoCreateRequest struct {
	Code         string
	Name         string
	Note         string
	BonusBytes   int64
	MaxUses      int
	ExpiresAfter time.Duration
}

type InviteRedeemResult struct {
	Invite InviteRecord
	Client ClientRecord
}

type PromoRedeemResult struct {
	Promo          PromoRecord
	Client         ClientRecord
	ActivationMode PromoActivationMode
}

type TrafficUsage struct {
	TotalBytes int64
	ObservedAt time.Time
}

type TrafficSyncResult struct {
	UpdatedClients   int
	BlockedClients   int
	UnblockedClients int
	TotalDeltaBytes  int64
	RequiresRefresh  bool
}

func (c ClientRecord) Enabled() bool {
	return c.Active && c.RevokedAt == nil && c.TrafficBlockedAt == nil && !c.AccessExpired(time.Now().UTC())
}

func (c ClientRecord) Bound() bool {
	return c.Active && c.RevokedAt == nil
}

func (c ClientRecord) AccessExpired(now time.Time) bool {
	return c.AccessExpiresAt != nil && now.After(*c.AccessExpiresAt)
}

func (c ClientRecord) TrafficLimited() bool {
	return c.TrafficLimitBytes > 0
}

func (c ClientRecord) TrafficRemainingBytes() int64 {
	if c.TrafficLimitBytes <= 0 {
		return 0
	}
	remaining := c.TrafficLimitBytes - c.TrafficUsedBytes
	if remaining < 0 {
		return 0
	}
	return remaining
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
	registry.normalize()

	previousPayload, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return Registry{}, fmt.Errorf("marshal registry: %w", err)
	}

	if err := mutator(&registry); err != nil {
		return Registry{}, err
	}

	registry.normalize()
	payload, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return Registry{}, fmt.Errorf("marshal registry: %w", err)
	}
	if bytes.Equal(previousPayload, payload) {
		return registry, nil
	}
	payload = append(payload, '\n')

	if err := writeFileAtomically(s.path, payload, 0o600); err != nil {
		return Registry{}, fmt.Errorf("write registry: %w", err)
	}

	if s.logger != nil {
		s.logger.Info(
			"registry updated",
			"path", s.path,
			"clients", len(registry.Clients),
			"invites", len(registry.Invites),
			"promos", len(registry.Promos),
		)
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
		bootstrap.TrafficBlockedAt = nil
		bootstrap.DeviceName = bootstrapDeviceName(bootstrap.DeviceName)
		return nil
	})
}

func (s *RegistryStore) Summary(cfg config.RealityConfig) (RegistrySummary, error) {
	registry, err := s.Load()
	if err != nil {
		return RegistrySummary{}, err
	}

	now := time.Now().UTC()
	summary := RegistrySummary{
		Server:            snapshotServer(cfg),
		BootstrapClientID: registry.BootstrapClientID,
		TotalClients:      len(registry.Clients),
		TotalInvites:      len(registry.Invites),
		TotalPromos:       len(registry.Promos),
	}
	for _, client := range registry.Clients {
		summary.TotalTrafficBytes += client.TrafficUsedBytes
		switch {
		case client.Enabled():
			summary.ActiveClients++
		case client.Bound() && client.TrafficBlockedAt != nil:
			summary.TrafficBlockedClients++
		default:
			summary.RevokedClients++
		}
	}
	for _, invite := range registry.Invites {
		if invite.isRedeemable(now) {
			summary.PendingInvites++
		}
		if invite.RedeemedUses > 0 {
			summary.RedeemedInvites++
		}
	}
	for _, promo := range registry.Promos {
		if promo.isRedeemable(now) {
			summary.ActivePromos++
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

func (s *RegistryStore) ObserveSubscriptionDevice(clientUUID string, deviceID string, observation SubscriptionDeviceObservation) (ClientRecord, bool, error) {
	normalizedUUID := strings.TrimSpace(clientUUID)
	normalizedLookupDeviceID := strings.TrimSpace(deviceID)
	normalizedObservedDeviceID := strings.TrimSpace(observation.DeviceID)
	if normalizedObservedDeviceID == "" {
		return ClientRecord{}, false, errors.New("device_id is required")
	}
	if normalizedUUID == "" && normalizedLookupDeviceID == "" {
		return ClientRecord{}, false, errors.New("client_uuid or device_id is required")
	}

	var (
		updated ClientRecord
		changed bool
	)
	_, err := s.Update(func(registry *Registry) error {
		var client *ClientRecord
		if normalizedUUID != "" {
			client = registry.findBoundClientByUUID(normalizedUUID)
		} else {
			client = registry.findBoundClientByDeviceID(normalizedLookupDeviceID)
		}
		if client == nil {
			return errors.New("client not found")
		}
		if client.DeviceID == normalizedObservedDeviceID {
			updated = *client
			return nil
		}

		seenAt := observation.SeenAt
		if seenAt.IsZero() {
			seenAt = time.Now().UTC()
		}
		deviceName := strings.TrimSpace(observation.DeviceName)
		deviceOS := strings.TrimSpace(observation.DeviceOS)
		deviceOSVersion := strings.TrimSpace(observation.DeviceOSVersion)
		userAgent := strings.TrimSpace(observation.UserAgent)

		for index := range client.ObservedDevices {
			record := &client.ObservedDevices[index]
			if record.DeviceID != normalizedObservedDeviceID {
				continue
			}
			if deviceName != "" && record.DeviceName != deviceName {
				record.DeviceName = deviceName
				changed = true
			}
			if deviceOS != "" && record.DeviceOS != deviceOS {
				record.DeviceOS = deviceOS
				changed = true
			}
			if deviceOSVersion != "" && record.DeviceOSVersion != deviceOSVersion {
				record.DeviceOSVersion = deviceOSVersion
				changed = true
			}
			if userAgent != "" && record.UserAgent != userAgent {
				record.UserAgent = userAgent
				changed = true
			}
			if record.LastSeenAt.Before(seenAt) {
				record.LastSeenAt = seenAt
				changed = true
			}
			updated = *client
			return nil
		}

		client.ObservedDevices = append(client.ObservedDevices, ObservedDeviceRecord{
			DeviceID:        normalizedObservedDeviceID,
			DeviceName:      deviceName,
			DeviceOS:        deviceOS,
			DeviceOSVersion: deviceOSVersion,
			UserAgent:       userAgent,
			FirstSeenAt:     seenAt,
			LastSeenAt:      seenAt,
		})
		sort.SliceStable(client.ObservedDevices, func(i, j int) bool {
			return client.ObservedDevices[i].FirstSeenAt.Before(client.ObservedDevices[j].FirstSeenAt)
		})
		changed = true
		updated = *client
		return nil
	})
	return updated, changed, err
}

func (s *RegistryStore) BlacklistClientDevices(clientID string, reason string, source string) (BlacklistResult, error) {
	normalizedClientID := strings.TrimSpace(clientID)
	if normalizedClientID == "" {
		return BlacklistResult{}, errors.New("client_id is required")
	}

	var result BlacklistResult
	_, err := s.Update(func(registry *Registry) error {
		client := registry.findClient(normalizedClientID)
		if client == nil {
			return errors.New("client not found")
		}

		now := time.Now().UTC()
		normalizedReason := strings.TrimSpace(reason)
		normalizedSource := strings.TrimSpace(source)
		blocked := make([]BlockedDeviceRecord, 0, 1+len(client.ObservedDevices))
		seen := make(map[string]struct{}, 1+len(client.ObservedDevices))

		addBlocked := func(deviceID string, deviceName string) {
			deviceID = strings.TrimSpace(deviceID)
			if deviceID == "" {
				return
			}
			if _, ok := seen[deviceID]; ok {
				return
			}
			seen[deviceID] = struct{}{}

			existing := registry.findBlockedDevice(deviceID)
			if existing == nil {
				registry.BlockedDevices = append(registry.BlockedDevices, BlockedDeviceRecord{
					DeviceID:   deviceID,
					DeviceName: strings.TrimSpace(deviceName),
					ClientID:   client.ID,
					ClientUUID: client.UUID,
					Reason:     normalizedReason,
					Source:     normalizedSource,
					BlockedAt:  now,
				})
				existing = &registry.BlockedDevices[len(registry.BlockedDevices)-1]
			} else {
				if existing.DeviceName == "" {
					existing.DeviceName = strings.TrimSpace(deviceName)
				}
				if existing.ClientID == "" {
					existing.ClientID = client.ID
				}
				if existing.ClientUUID == "" {
					existing.ClientUUID = client.UUID
				}
				if normalizedReason != "" {
					existing.Reason = normalizedReason
				}
				if normalizedSource != "" {
					existing.Source = normalizedSource
				}
				if existing.BlockedAt.IsZero() {
					existing.BlockedAt = now
				}
			}
			blocked = append(blocked, *existing)
		}

		addBlocked(client.DeviceID, client.DeviceName)
		for _, observed := range client.ObservedDevices {
			addBlocked(observed.DeviceID, observed.DeviceName)
		}

		if invite := registry.findInvite(client.InviteCode); invite != nil {
			invite.Active = false
			expiredAt := now
			invite.ExpiresAt = &expiredAt
		}

		client.Active = false
		client.RevokedAt = &now
		client.TrafficBlockedAt = nil
		client.UpdatedAt = now

		result = BlacklistResult{
			Client:         *client,
			BlockedDevices: blocked,
		}
		return nil
	})
	return result, err
}

func (s *RegistryStore) DeactivateInvite(code string) (InviteRecord, error) {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		return InviteRecord{}, errors.New("invite_code is required")
	}

	var updated InviteRecord
	_, err := s.Update(func(registry *Registry) error {
		invite := registry.findInvite(normalizedCode)
		if invite == nil {
			return errors.New("invite not found")
		}
		now := time.Now().UTC()
		invite.Active = false
		invite.ExpiresAt = &now
		updated = *invite
		return nil
	})
	return updated, err
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

func (s *RegistryStore) ListPromos() ([]PromoRecord, error) {
	registry, err := s.Load()
	if err != nil {
		return nil, err
	}
	promos := append([]PromoRecord(nil), registry.Promos...)
	sort.SliceStable(promos, func(i, j int) bool {
		return promos[i].CreatedAt.Before(promos[j].CreatedAt)
	})
	return promos, nil
}

func (s *RegistryStore) CreateInvite(input InviteCreateRequest) (InviteRecord, error) {
	var created InviteRecord
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		created = InviteRecord{
			Code:               generateRegistryToken("inv"),
			Name:               firstNonEmpty(strings.TrimSpace(input.Name), "New device"),
			Note:               strings.TrimSpace(input.Note),
			PlanID:             strings.TrimSpace(input.PlanID),
			PlanName:           strings.TrimSpace(input.PlanName),
			AllowedServerIDs:   normalizeStringSlice(input.AllowedServerIDs),
			AccessDurationDays: normalizeInviteAccessDays(input.AccessDurationDays),
			MaxUses:            normalizeInviteMaxUses(input.MaxUses),
			TrafficLimitBytes:  normalizeTrafficBytes(input.TrafficLimitBytes),
			CreatedAt:          now,
			Active:             true,
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

func (s *RegistryStore) CreatePromo(input PromoCreateRequest) (PromoRecord, error) {
	var created PromoRecord
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		promoCode := strings.TrimSpace(input.Code)
		if promoCode == "" {
			promoCode = generateRegistryToken("promo")
			for registry.hasCode(promoCode) {
				promoCode = generateRegistryToken("promo")
			}
		} else {
			normalizedCode, err := normalizeCustomPromoCode(promoCode)
			if err != nil {
				return err
			}
			if registry.hasCode(normalizedCode) {
				return fmt.Errorf("code %q is already in use", normalizedCode)
			}
			promoCode = normalizedCode
		}

		created = PromoRecord{
			Code:       promoCode,
			Name:       firstNonEmpty(strings.TrimSpace(input.Name), "Free traffic"),
			Note:       strings.TrimSpace(input.Note),
			BonusBytes: normalizeTrafficBytes(input.BonusBytes),
			MaxUses:    normalizePromoMaxUses(input.MaxUses),
			CreatedAt:  now,
			Active:     true,
		}
		if input.ExpiresAfter > 0 {
			expiresAt := now.Add(input.ExpiresAfter)
			created.ExpiresAt = &expiresAt
		}
		registry.Promos = append(registry.Promos, created)
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
		if !invite.isRedeemable(now) {
			return invite.redeemError(now)
		}

		normalizedDeviceID := strings.TrimSpace(deviceID)
		if normalizedDeviceID == "" {
			normalizedDeviceID = generateRegistryToken("device")
		}
		if blocked := registry.findBlockedDevice(normalizedDeviceID); blocked != nil {
			return fmt.Errorf("device %q is blacklisted", normalizedDeviceID)
		}
		normalizedDeviceName := firstNonEmpty(strings.TrimSpace(deviceName), "Imported device")

		if existing := registry.findBoundClientByDeviceID(normalizedDeviceID); existing != nil {
			if existing.InviteCode != invite.Code {
				return fmt.Errorf("device %q already has an active profile", normalizedDeviceID)
			}
			existing.DeviceName = normalizedDeviceName
			existing.Name = firstNonEmpty(strings.TrimSpace(existing.Name), strings.TrimSpace(invite.Name), normalizedDeviceName)
			existing.PlanID = strings.TrimSpace(invite.PlanID)
			existing.PlanName = strings.TrimSpace(invite.PlanName)
			existing.AllowedServerIDs = append([]string(nil), invite.AllowedServerIDs...)
			if invite.AccessDurationDays > 0 {
				expiresAt := now.Add(time.Duration(invite.AccessDurationDays) * 24 * time.Hour)
				existing.AccessExpiresAt = &expiresAt
			}
			existing.UpdatedAt = now
			bindInviteRedemption(invite, *existing, now)
			result = InviteRedeemResult{
				Invite: *invite,
				Client: *existing,
			}
			return nil
		}

		clientUUID, err := generateUUID()
		if err != nil {
			return err
		}

		clientID := generateRegistryToken("client")
		client := ClientRecord{
			ID:                clientID,
			Name:              firstNonEmpty(strings.TrimSpace(invite.Name), normalizedDeviceName, "Imported device"),
			DeviceID:          normalizedDeviceID,
			DeviceName:        normalizedDeviceName,
			UUID:              clientUUID,
			Email:             buildClientEmail(clientID, invite.Name, normalizedDeviceName, normalizedDeviceID),
			InviteCode:        invite.Code,
			PlanID:            strings.TrimSpace(invite.PlanID),
			PlanName:          strings.TrimSpace(invite.PlanName),
			AllowedServerIDs:  append([]string(nil), invite.AllowedServerIDs...),
			CreatedAt:         now,
			UpdatedAt:         now,
			TrafficLimitBytes: normalizeTrafficBytes(invite.TrafficLimitBytes),
			Active:            true,
		}
		if invite.AccessDurationDays > 0 {
			expiresAt := now.Add(time.Duration(invite.AccessDurationDays) * 24 * time.Hour)
			client.AccessExpiresAt = &expiresAt
		}

		invite.RedeemedUses++
		bindInviteRedemption(invite, client, now)

		registry.Clients = append(registry.Clients, client)
		result = InviteRedeemResult{
			Invite: *invite,
			Client: client,
		}
		return nil
	})
	return result, err
}

func (s *RegistryStore) RedeemPromo(code string, deviceID string, deviceName string) (PromoRedeemResult, error) {
	var result PromoRedeemResult
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		promo := registry.findPromo(strings.TrimSpace(strings.ToLower(code)))
		if promo == nil {
			return errors.New("promo code not found")
		}
		if !promo.isRedeemable(now) {
			return promo.redeemError(now)
		}

		normalizedDeviceID := strings.TrimSpace(deviceID)
		if normalizedDeviceID == "" {
			return errors.New("device_id is required for promo activation")
		}
		if blocked := registry.findBlockedDevice(normalizedDeviceID); blocked != nil {
			return fmt.Errorf("device %q is blacklisted", normalizedDeviceID)
		}
		if promo.hasDevice(normalizedDeviceID) {
			return errors.New("promo code was already activated on this device")
		}

		normalizedDeviceName := firstNonEmpty(strings.TrimSpace(deviceName), "Android device")
		client := registry.findBoundClientByDeviceID(normalizedDeviceID)
		activationMode := PromoActivationModeBonus

		if client == nil {
			clientUUID, err := generateUUID()
			if err != nil {
				return err
			}
			clientID := generateRegistryToken("client")
			trafficLimit := normalizeTrafficBytes(promo.BonusBytes)
			clientRecord := ClientRecord{
				ID:                clientID,
				Name:              firstNonEmpty(strings.TrimSpace(promo.Name), normalizedDeviceName, "Trial device"),
				DeviceID:          normalizedDeviceID,
				DeviceName:        normalizedDeviceName,
				UUID:              clientUUID,
				Email:             buildClientEmail(clientID, promo.Name, normalizedDeviceName, normalizedDeviceID),
				CreatedAt:         now,
				UpdatedAt:         now,
				TrafficLimitBytes: trafficLimit,
				TrafficBonusBytes: trafficLimit,
				Active:            true,
			}
			registry.Clients = append(registry.Clients, clientRecord)
			client = &registry.Clients[len(registry.Clients)-1]
			activationMode = PromoActivationModeTrial
		} else {
			normalizedDeviceName = firstNonEmpty(strings.TrimSpace(deviceName), client.DeviceName, "Android device")
			client.DeviceName = normalizedDeviceName
			client.UpdatedAt = now
			client.TrafficBonusBytes += promo.BonusBytes
			if client.TrafficLimitBytes > 0 {
				client.TrafficLimitBytes += promo.BonusBytes
			}
			if client.TrafficLimitBytes == 0 && promo.BonusBytes > 0 {
				client.TrafficLimitBytes = promo.BonusBytes
			}
		}
		if client.TrafficLimitBytes <= 0 || client.TrafficUsedBytes < client.TrafficLimitBytes {
			client.TrafficBlockedAt = nil
		}

		promo.Redemptions = append(promo.Redemptions, PromoRedemption{
			DeviceID:   normalizedDeviceID,
			DeviceName: normalizedDeviceName,
			ClientID:   client.ID,
			RedeemedAt: now,
			BonusBytes: promo.BonusBytes,
		})
		promo.RedeemedUses++
		promo.LastRedeemedAt = &now

		result = PromoRedeemResult{
			Promo:          *promo,
			Client:         *client,
			ActivationMode: activationMode,
		}
		return nil
	})
	return result, err
}

func (s *RegistryStore) RevokeClient(clientID string) (ClientRecord, error) {
	return s.deactivateClient(func(registry *Registry) *ClientRecord {
		return registry.findClient(clientID)
	})
}

func (s *RegistryStore) DisconnectDevice(deviceID string, clientUUID string) (ClientRecord, error) {
	normalizedDeviceID := strings.TrimSpace(deviceID)
	normalizedUUID := strings.TrimSpace(clientUUID)
	if normalizedDeviceID == "" {
		return ClientRecord{}, errors.New("device_id is required")
	}

	return s.deactivateClient(func(registry *Registry) *ClientRecord {
		return registry.findBoundClientByDeviceAndUUID(normalizedDeviceID, normalizedUUID)
	})
}

func (s *RegistryStore) deactivateClient(resolve func(*Registry) *ClientRecord) (ClientRecord, error) {
	var updated ClientRecord
	_, err := s.Update(func(registry *Registry) error {
		client := resolve(registry)
		if client == nil {
			return errors.New("client not found")
		}
		now := time.Now().UTC()
		client.Active = false
		client.RevokedAt = &now
		client.TrafficBlockedAt = nil
		client.UpdatedAt = now
		updated = *client
		return nil
	})
	return updated, err
}

func (s *RegistryStore) ApplyTrafficStats(usages map[string]TrafficUsage) (TrafficSyncResult, error) {
	var result TrafficSyncResult
	_, err := s.Update(func(registry *Registry) error {
		now := time.Now().UTC()
		for i := range registry.Clients {
			client := &registry.Clients[i]
			if usage, ok := usages[client.Email]; ok {
				delta := usage.TotalBytes - client.TrafficObservedBytes
				if delta < 0 {
					delta = usage.TotalBytes
				}
				if delta > 0 {
					client.TrafficUsedBytes += delta
					result.TotalDeltaBytes += delta
					result.UpdatedClients++
				}
				client.TrafficObservedBytes = usage.TotalBytes
				observedAt := usage.ObservedAt
				client.LastTrafficSyncAt = &observedAt
			}

			if !client.Bound() {
				continue
			}

			shouldBlock := client.TrafficLimitBytes > 0 && client.TrafficUsedBytes >= client.TrafficLimitBytes
			isBlocked := client.TrafficBlockedAt != nil
			switch {
			case shouldBlock && !isBlocked:
				client.TrafficBlockedAt = &now
				result.BlockedClients++
				result.RequiresRefresh = true
			case !shouldBlock && isBlocked:
				client.TrafficBlockedAt = nil
				result.UnblockedClients++
				result.RequiresRefresh = true
			}
		}
		return nil
	})
	return result, err
}

func (s *RegistryStore) PrimaryClient() (ClientRecord, error) {
	registry, err := s.Load()
	if err != nil {
		return ClientRecord{}, err
	}
	for _, client := range registry.Clients {
		if client.Enabled() {
			return client, nil
		}
	}
	return ClientRecord{}, errors.New("no active clients in registry")
}

func (r Registry) BoundClients() []ClientRecord {
	clients := make([]ClientRecord, 0, len(r.Clients))
	for _, client := range r.Clients {
		if client.Bound() {
			clients = append(clients, client)
		}
	}
	sort.SliceStable(clients, func(i, j int) bool {
		return clients[i].CreatedAt.Before(clients[j].CreatedAt)
	})
	return clients
}

func (r Registry) ActiveClients() []ClientRecord {
	clients := make([]ClientRecord, 0, len(r.Clients))
	for _, client := range r.Clients {
		if client.Enabled() {
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
	client, ok := r.primaryEnabledClient()
	if !ok {
		return ClientProfile{}, errors.New("no active client available")
	}
	return buildClientProfileFor(cfg, state, client), nil
}

func (r Registry) primaryEnabledClient() (ClientRecord, bool) {
	for _, client := range r.Clients {
		if client.Enabled() {
			return client, true
		}
	}
	return ClientRecord{}, false
}

func (r *Registry) normalize() {
	if r.Version < 2 {
		r.Version = 2
	}
	now := time.Now().UTC()
	activeInviteUses := make(map[string]int)
	seenEmails := make(map[string]struct{})

	for i := range r.Clients {
		client := &r.Clients[i]
		if client.DeviceID == "bootstrap" || client.ID == r.BootstrapClientID {
			client.DeviceName = bootstrapDeviceName(client.DeviceName)
		}
		client.Email = ensureUniqueClientEmail(*client, seenEmails)
		client.PlanID = strings.TrimSpace(client.PlanID)
		client.PlanName = strings.TrimSpace(client.PlanName)
		client.AllowedServerIDs = normalizeStringSlice(client.AllowedServerIDs)
		if client.TrafficLimitBytes < 0 {
			client.TrafficLimitBytes = 0
		}
		if client.TrafficBonusBytes < 0 {
			client.TrafficBonusBytes = 0
		}
		if client.TrafficUsedBytes < 0 {
			client.TrafficUsedBytes = 0
		}
		if client.TrafficObservedBytes < 0 {
			client.TrafficObservedBytes = 0
		}
		if client.Bound() {
			activeInviteUses[client.InviteCode]++
			if client.TrafficLimitBytes > 0 && client.TrafficUsedBytes >= client.TrafficLimitBytes {
				if client.TrafficBlockedAt == nil {
					blockedAt := now
					client.TrafficBlockedAt = &blockedAt
				}
			} else {
				client.TrafficBlockedAt = nil
			}
		} else {
			client.TrafficBlockedAt = nil
		}
	}

	for i := range r.Invites {
		r.Invites[i].PlanID = strings.TrimSpace(r.Invites[i].PlanID)
		r.Invites[i].PlanName = strings.TrimSpace(r.Invites[i].PlanName)
		r.Invites[i].AllowedServerIDs = normalizeStringSlice(r.Invites[i].AllowedServerIDs)
		r.Invites[i].AccessDurationDays = normalizeInviteAccessDays(r.Invites[i].AccessDurationDays)
		if r.Invites[i].MaxUses <= 0 {
			r.Invites[i].MaxUses = 1
		}
		if r.Invites[i].TrafficLimitBytes < 0 {
			r.Invites[i].TrafficLimitBytes = 0
		}
		r.Invites[i].ActiveUses = activeInviteUses[r.Invites[i].Code]
		if r.Invites[i].RedeemedUses < r.Invites[i].ActiveUses {
			r.Invites[i].RedeemedUses = r.Invites[i].ActiveUses
		}
		r.Invites[i].Active = r.Invites[i].isRedeemable(now)
	}

	for i := range r.Promos {
		r.Promos[i].Code = strings.TrimSpace(strings.ToLower(r.Promos[i].Code))
		if r.Promos[i].BonusBytes < 0 {
			r.Promos[i].BonusBytes = 0
		}
		r.Promos[i].MaxUses = normalizePromoMaxUses(r.Promos[i].MaxUses)
		if r.Promos[i].RedeemedUses < len(r.Promos[i].Redemptions) {
			r.Promos[i].RedeemedUses = len(r.Promos[i].Redemptions)
		}
		r.Promos[i].Active = r.Promos[i].isRedeemable(now)
		sort.SliceStable(r.Promos[i].Redemptions, func(a, b int) bool {
			return r.Promos[i].Redemptions[a].RedeemedAt.Before(r.Promos[i].Redemptions[b].RedeemedAt)
		})
	}
	for i := range r.BlockedDevices {
		r.BlockedDevices[i].DeviceID = strings.TrimSpace(r.BlockedDevices[i].DeviceID)
		r.BlockedDevices[i].DeviceName = strings.TrimSpace(r.BlockedDevices[i].DeviceName)
		r.BlockedDevices[i].ClientID = strings.TrimSpace(r.BlockedDevices[i].ClientID)
		r.BlockedDevices[i].ClientUUID = strings.TrimSpace(r.BlockedDevices[i].ClientUUID)
		r.BlockedDevices[i].Reason = strings.TrimSpace(r.BlockedDevices[i].Reason)
		r.BlockedDevices[i].Source = strings.TrimSpace(r.BlockedDevices[i].Source)
		if r.BlockedDevices[i].BlockedAt.IsZero() {
			r.BlockedDevices[i].BlockedAt = now
		}
	}
	sort.SliceStable(r.Invites, func(i, j int) bool {
		return r.Invites[i].CreatedAt.Before(r.Invites[j].CreatedAt)
	})
	sort.SliceStable(r.Promos, func(i, j int) bool {
		return r.Promos[i].CreatedAt.Before(r.Promos[j].CreatedAt)
	})
	sort.SliceStable(r.Clients, func(i, j int) bool {
		return r.Clients[i].CreatedAt.Before(r.Clients[j].CreatedAt)
	})
	sort.SliceStable(r.BlockedDevices, func(i, j int) bool {
		if r.BlockedDevices[i].BlockedAt.Equal(r.BlockedDevices[j].BlockedAt) {
			return r.BlockedDevices[i].DeviceID < r.BlockedDevices[j].DeviceID
		}
		return r.BlockedDevices[i].BlockedAt.Before(r.BlockedDevices[j].BlockedAt)
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

func (r *Registry) findPromo(code string) *PromoRecord {
	needle := strings.TrimSpace(strings.ToLower(code))
	if needle == "" {
		return nil
	}
	for i := range r.Promos {
		if strings.TrimSpace(strings.ToLower(r.Promos[i].Code)) == needle {
			return &r.Promos[i]
		}
	}
	return nil
}

func (r *Registry) hasCode(code string) bool {
	needle := strings.TrimSpace(strings.ToLower(code))
	if needle == "" {
		return false
	}
	for _, invite := range r.Invites {
		if strings.TrimSpace(strings.ToLower(invite.Code)) == needle {
			return true
		}
	}
	for _, promo := range r.Promos {
		if strings.TrimSpace(strings.ToLower(promo.Code)) == needle {
			return true
		}
	}
	return false
}

func (r *Registry) findClient(id string) *ClientRecord {
	for i := range r.Clients {
		if r.Clients[i].ID == id {
			return &r.Clients[i]
		}
	}
	return nil
}

func (r *Registry) findBoundClientByDeviceID(deviceID string) *ClientRecord {
	for i := range r.Clients {
		client := &r.Clients[i]
		if client.DeviceID == deviceID && client.Bound() {
			return client
		}
	}
	return nil
}

func (r *Registry) findBoundClientByUUID(clientUUID string) *ClientRecord {
	for index := range r.Clients {
		client := &r.Clients[index]
		if client.UUID == clientUUID && client.Bound() {
			return client
		}
	}
	return nil
}

func (r *Registry) findBlockedDevice(deviceID string) *BlockedDeviceRecord {
	for index := range r.BlockedDevices {
		blocked := &r.BlockedDevices[index]
		if blocked.DeviceID == deviceID {
			return blocked
		}
	}
	return nil
}

func (r *Registry) findBoundClientByDeviceAndUUID(deviceID string, clientUUID string) *ClientRecord {
	for i := range r.Clients {
		client := &r.Clients[i]
		if client.DeviceID != deviceID || !client.Bound() {
			continue
		}
		if clientUUID == "" || client.UUID == clientUUID {
			return client
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
		ServerID:    "primary",
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
		Location:    cfg.PublicHost,
	}
}

func buildClientProfilesFor(cfg config.RealityConfig, state State, client ClientRecord) []ClientProfile {
	profiles := []ClientProfile{
		buildClientProfileFor(cfg, state, client),
	}

	for index, additional := range cfg.AdditionalServers {
		address := strings.TrimSpace(additional.PublicHost)
		if address == "" || additional.PublicPort <= 0 {
			continue
		}

		serverName := ""
		if len(additional.ServerNames) > 0 {
			serverName = strings.TrimSpace(additional.ServerNames[0])
		}
		if serverName == "" && len(cfg.ServerNames) > 0 {
			serverName = strings.TrimSpace(cfg.ServerNames[0])
		}

		shortIDs := append([]string(nil), additional.ShortIDs...)
		if len(shortIDs) == 0 {
			shortIDs = append([]string(nil), state.ShortIDs...)
		}
		shortID := strings.TrimSpace(additional.ShortID)
		if shortID == "" && len(shortIDs) > 0 {
			shortID = strings.TrimSpace(shortIDs[0])
		}

		name := client.Name
		if strings.TrimSpace(name) == "" {
			name = client.DeviceName
		}
		if strings.TrimSpace(name) == "" {
			name = "novpn-device"
		}

		nodeName := strings.TrimSpace(additional.Name)
		if nodeName == "" {
			nodeName = fmt.Sprintf("VPN node %d", index+2)
		}
		if additional.VPNOnly {
			nodeName += " (VPN)"
		}
		profileName := fmt.Sprintf("%s · %s", name, nodeName)

		profiles = append(profiles, ClientProfile{
			GeneratedAt: time.Now().UTC(),
			Name:        profileName,
			Type:        "vless-reality",
			ServerID:    fmt.Sprintf("additional-%d", index+1),
			Address:     address,
			Port:        additional.PublicPort,
			UUID:        client.UUID,
			Flow:        additional.Flow,
			Network:     "tcp",
			Security:    "reality",
			ServerName:  serverName,
			Fingerprint: additional.Fingerprint,
			PublicKey:   firstNonEmpty(strings.TrimSpace(additional.PublicKey), strings.TrimSpace(state.PublicKey)),
			ShortID:     shortID,
			ShortIDs:    shortIDs,
			SpiderX:     additional.SpiderX,
			Location:    strings.TrimSpace(additional.Name),
		})
	}

	return profiles
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeInviteMaxUses(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeInviteAccessDays(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func normalizePromoMaxUses(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func normalizeTrafficBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func (i InviteRecord) remainingUses() int {
	maxUses := normalizeInviteMaxUses(i.MaxUses)
	remaining := maxUses - i.ActiveUses
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (i InviteRecord) isRedeemable(now time.Time) bool {
	if !i.Active {
		return false
	}
	if i.ExpiresAt != nil && now.After(*i.ExpiresAt) {
		return false
	}
	return i.remainingUses() > 0
}

func (i InviteRecord) redeemError(now time.Time) error {
	switch {
	case !i.Active:
		return errors.New("invite is inactive")
	case i.ExpiresAt != nil && now.After(*i.ExpiresAt):
		return errors.New("invite expired")
	case i.remainingUses() == 0:
		return errors.New("invite usage limit reached")
	default:
		return errors.New("invite is inactive")
	}
}

func (p PromoRecord) isRedeemable(now time.Time) bool {
	if !p.Active {
		return false
	}
	if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
		return false
	}
	if p.BonusBytes <= 0 {
		return false
	}
	if p.MaxUses > 0 && p.RedeemedUses >= p.MaxUses {
		return false
	}
	return true
}

func (p PromoRecord) redeemError(now time.Time) error {
	switch {
	case p.ExpiresAt != nil && now.After(*p.ExpiresAt):
		return errors.New("promo code expired")
	case p.BonusBytes <= 0:
		return errors.New("promo code has no traffic bonus configured")
	case p.MaxUses > 0 && p.RedeemedUses >= p.MaxUses:
		return errors.New("promo usage limit reached")
	default:
		return errors.New("promo code is inactive")
	}
}

func (p PromoRecord) hasDevice(deviceID string) bool {
	for _, redemption := range p.Redemptions {
		if redemption.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func bindInviteRedemption(invite *InviteRecord, client ClientRecord, redeemedAt time.Time) {
	invite.RedeemedAt = &redeemedAt
	invite.RedeemedClientID = client.ID
	invite.RedeemedDeviceID = client.DeviceID
	invite.RedeemedDeviceName = client.DeviceName
}

func ensureUniqueClientEmail(client ClientRecord, seen map[string]struct{}) string {
	candidate := strings.TrimSpace(client.Email)
	if candidate == "" {
		candidate = buildClientEmail(client.ID, client.Name, client.DeviceName, client.DeviceID)
	}
	if _, exists := seen[candidate]; !exists {
		seen[candidate] = struct{}{}
		return candidate
	}

	fallback := buildClientEmail(client.ID, client.DeviceID)
	if _, exists := seen[fallback]; !exists {
		seen[fallback] = struct{}{}
		return fallback
	}

	index := 1
	for {
		next := strings.TrimSuffix(fallback, "@novpn") + fmt.Sprintf("-%d@novpn", index)
		if _, exists := seen[next]; !exists {
			seen[next] = struct{}{}
			return next
		}
		index++
	}
}

func buildClientEmail(parts ...string) string {
	return slugEmail(parts...)
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

func normalizeCustomPromoCode(raw string) (string, error) {
	code := strings.TrimSpace(strings.ToLower(raw))
	if code == "" {
		return "", errors.New("promo code is empty")
	}
	if len(code) < 4 || len(code) > 64 {
		return "", errors.New("promo code must be 4..64 characters")
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return "", errors.New("promo code supports only [a-z0-9-_.]")
		}
	}
	return code, nil
}

func (s *RegistryStore) MergeRemote(remote Registry) (bool, Registry, error) {
	var changed bool
	snapshot, err := s.Update(func(local *Registry) error {
		local.normalize()
		remote.normalize()

		before, err := json.Marshal(runtimeComparableRegistry(*local))
		if err != nil {
			return err
		}

		merged := mergeRegistrySnapshots(*local, remote)
		after, err := json.Marshal(runtimeComparableRegistry(merged))
		if err != nil {
			return err
		}

		changed = !bytes.Equal(before, after)
		*local = merged
		return nil
	})
	return changed, snapshot, err
}

func mergeRegistrySnapshots(local Registry, remote Registry) Registry {
	merged := remote
	if local.Server != (RegistryServer{}) {
		merged.Server = local.Server
	}
	localClients := make(map[string]ClientRecord, len(local.Clients))
	for _, client := range local.Clients {
		localClients[client.ID] = client
	}

	for index := range merged.Clients {
		remoteClient := &merged.Clients[index]
		localClient, ok := localClients[remoteClient.ID]
		if !ok {
			continue
		}
		if remoteClient.ID == local.BootstrapClientID || remoteClient.DeviceID == "bootstrap" {
			preserveBootstrapClient(remoteClient, localClient)
			continue
		}
		if localClient.TrafficUsedBytes > remoteClient.TrafficUsedBytes {
			remoteClient.TrafficUsedBytes = localClient.TrafficUsedBytes
		}
		if localClient.TrafficObservedBytes > remoteClient.TrafficObservedBytes {
			remoteClient.TrafficObservedBytes = localClient.TrafficObservedBytes
		}
		if remoteClient.LastTrafficSyncAt == nil || (localClient.LastTrafficSyncAt != nil && localClient.LastTrafficSyncAt.After(*remoteClient.LastTrafficSyncAt)) {
			remoteClient.LastTrafficSyncAt = localClient.LastTrafficSyncAt
		}
		if remoteClient.LastSeenAt == nil {
			remoteClient.LastSeenAt = localClient.LastSeenAt
		}
		if remoteClient.AccessExpiresAt == nil {
			remoteClient.AccessExpiresAt = localClient.AccessExpiresAt
		}
		remoteClient.ObservedDevices = mergeObservedDevices(localClient.ObservedDevices, remoteClient.ObservedDevices)
	}

	merged.BlockedDevices = mergeBlockedDevices(local.BlockedDevices, merged.BlockedDevices)

	if merged.BootstrapClientID == "" {
		merged.BootstrapClientID = local.BootstrapClientID
	}
	if merged.BootstrapClientID != "" && merged.findClient(merged.BootstrapClientID) == nil {
		if bootstrap := local.findClient(merged.BootstrapClientID); bootstrap != nil {
			merged.Clients = append(merged.Clients, *bootstrap)
		}
	}
	return merged
}

func preserveBootstrapClient(target *ClientRecord, source ClientRecord) {
	target.ID = source.ID
	target.Name = source.Name
	target.DeviceID = source.DeviceID
	target.DeviceName = source.DeviceName
	target.ObservedDevices = append([]ObservedDeviceRecord(nil), source.ObservedDevices...)
	target.UUID = source.UUID
	target.Email = source.Email
	target.InviteCode = source.InviteCode
	target.PlanID = source.PlanID
	target.PlanName = source.PlanName
	target.AllowedServerIDs = append([]string(nil), source.AllowedServerIDs...)
	target.CreatedAt = source.CreatedAt
	target.UpdatedAt = source.UpdatedAt
	target.RevokedAt = source.RevokedAt
	target.LastSeenAt = source.LastSeenAt
	target.AccessExpiresAt = source.AccessExpiresAt
	target.TrafficLimitBytes = source.TrafficLimitBytes
	target.TrafficBonusBytes = source.TrafficBonusBytes
	target.TrafficUsedBytes = source.TrafficUsedBytes
	target.TrafficObservedBytes = source.TrafficObservedBytes
	target.LastTrafficSyncAt = source.LastTrafficSyncAt
	target.TrafficBlockedAt = source.TrafficBlockedAt
	target.Active = source.Active
}

func mergeObservedDevices(local []ObservedDeviceRecord, remote []ObservedDeviceRecord) []ObservedDeviceRecord {
	if len(local) == 0 && len(remote) == 0 {
		return nil
	}

	merged := make(map[string]ObservedDeviceRecord, len(local)+len(remote))
	for _, record := range remote {
		key := strings.TrimSpace(record.DeviceID)
		if key == "" {
			continue
		}
		merged[key] = record
	}
	for _, record := range local {
		key := strings.TrimSpace(record.DeviceID)
		if key == "" {
			continue
		}
		existing, ok := merged[key]
		if !ok {
			merged[key] = record
			continue
		}
		merged[key] = mergeObservedDeviceRecord(existing, record)
	}

	result := make([]ObservedDeviceRecord, 0, len(merged))
	for _, record := range merged {
		result = append(result, record)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].FirstSeenAt.Before(result[j].FirstSeenAt)
	})
	return result
}

func mergeObservedDeviceRecord(base ObservedDeviceRecord, candidate ObservedDeviceRecord) ObservedDeviceRecord {
	if base.DeviceName == "" {
		base.DeviceName = candidate.DeviceName
	}
	if base.DeviceOS == "" {
		base.DeviceOS = candidate.DeviceOS
	}
	if base.DeviceOSVersion == "" {
		base.DeviceOSVersion = candidate.DeviceOSVersion
	}
	if base.UserAgent == "" {
		base.UserAgent = candidate.UserAgent
	}
	if base.FirstSeenAt.IsZero() || (!candidate.FirstSeenAt.IsZero() && candidate.FirstSeenAt.Before(base.FirstSeenAt)) {
		base.FirstSeenAt = candidate.FirstSeenAt
	}
	if candidate.LastSeenAt.After(base.LastSeenAt) {
		base.LastSeenAt = candidate.LastSeenAt
	}
	return base
}

func mergeBlockedDevices(local []BlockedDeviceRecord, remote []BlockedDeviceRecord) []BlockedDeviceRecord {
	if len(local) == 0 && len(remote) == 0 {
		return nil
	}

	merged := make(map[string]BlockedDeviceRecord, len(local)+len(remote))
	for _, record := range remote {
		key := strings.TrimSpace(record.DeviceID)
		if key == "" {
			continue
		}
		merged[key] = record
	}
	for _, record := range local {
		key := strings.TrimSpace(record.DeviceID)
		if key == "" {
			continue
		}
		existing, ok := merged[key]
		if !ok {
			merged[key] = record
			continue
		}
		merged[key] = mergeBlockedDeviceRecord(existing, record)
	}

	result := make([]BlockedDeviceRecord, 0, len(merged))
	for _, record := range merged {
		result = append(result, record)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].BlockedAt.Before(result[j].BlockedAt)
	})
	return result
}

func mergeBlockedDeviceRecord(base BlockedDeviceRecord, candidate BlockedDeviceRecord) BlockedDeviceRecord {
	if base.DeviceName == "" {
		base.DeviceName = candidate.DeviceName
	}
	if base.ClientID == "" {
		base.ClientID = candidate.ClientID
	}
	if base.ClientUUID == "" {
		base.ClientUUID = candidate.ClientUUID
	}
	if base.Reason == "" {
		base.Reason = candidate.Reason
	}
	if base.Source == "" {
		base.Source = candidate.Source
	}
	if base.BlockedAt.IsZero() || (!candidate.BlockedAt.IsZero() && candidate.BlockedAt.Before(base.BlockedAt)) {
		base.BlockedAt = candidate.BlockedAt
	}
	return base
}

type runtimeComparableRegistrySnapshot struct {
	BootstrapClientID string                            `json:"bootstrap_client_id,omitempty"`
	Clients           []runtimeComparableClientSnapshot `json:"clients,omitempty"`
	BlockedDevices    []BlockedDeviceRecord             `json:"blocked_devices,omitempty"`
}

type runtimeComparableClientSnapshot struct {
	ID               string     `json:"id"`
	UUID             string     `json:"uuid"`
	Email            string     `json:"email"`
	CreatedAt        time.Time  `json:"created_at"`
	Active           bool       `json:"active"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	AccessExpiresAt  *time.Time `json:"access_expires_at,omitempty"`
	TrafficBlockedAt *time.Time `json:"traffic_blocked_at,omitempty"`
}

func runtimeComparableRegistry(registry Registry) runtimeComparableRegistrySnapshot {
	comparable := runtimeComparableRegistrySnapshot{
		BootstrapClientID: strings.TrimSpace(registry.BootstrapClientID),
		Clients:           make([]runtimeComparableClientSnapshot, 0, len(registry.Clients)),
		BlockedDevices:    append([]BlockedDeviceRecord(nil), registry.BlockedDevices...),
	}
	for _, client := range registry.Clients {
		comparable.Clients = append(comparable.Clients, runtimeComparableClientSnapshot{
			ID:               client.ID,
			UUID:             client.UUID,
			Email:            client.Email,
			CreatedAt:        client.CreatedAt,
			Active:           client.Active,
			RevokedAt:        client.RevokedAt,
			AccessExpiresAt:  client.AccessExpiresAt,
			TrafficBlockedAt: client.TrafficBlockedAt,
		})
	}
	return comparable
}
