package reality

import (
	"novpn/internal/config"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreatePromoSupportsCustomCodeAndMaxUses(t *testing.T) {
	store := newTestRegistryStore(t)

	invite, err := store.CreateInvite(InviteCreateRequest{
		Name:    "test invite",
		MaxUses: 3,
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	devices := []string{"device-1", "device-2", "device-3"}
	for _, device := range devices {
		if _, err := store.RedeemInvite(invite.Code, device, device); err != nil {
			t.Fatalf("redeem invite for %s: %v", device, err)
		}
	}

	promo, err := store.CreatePromo(PromoCreateRequest{
		Code:       "SPRING_2026",
		Name:       "Spring promo",
		BonusBytes: 10 * 1024 * 1024,
		MaxUses:    2,
	})
	if err != nil {
		t.Fatalf("create promo: %v", err)
	}

	if promo.Code != "spring_2026" {
		t.Fatalf("unexpected promo code normalization: got %q", promo.Code)
	}

	for i := 0; i < 2; i++ {
		if _, err := store.RedeemPromo(promo.Code, devices[i], devices[i]); err != nil {
			t.Fatalf("redeem promo for %s: %v", devices[i], err)
		}
	}

	_, err = store.RedeemPromo(promo.Code, devices[2], devices[2])
	if err == nil || !strings.Contains(err.Error(), "usage limit reached") {
		t.Fatalf("expected usage limit error, got: %v", err)
	}

	promos, err := store.ListPromos()
	if err != nil {
		t.Fatalf("list promos: %v", err)
	}
	if len(promos) != 1 {
		t.Fatalf("expected 1 promo, got %d", len(promos))
	}
	if promos[0].RedeemedUses != 2 {
		t.Fatalf("expected 2 redemptions, got %d", promos[0].RedeemedUses)
	}
	if promos[0].Active {
		t.Fatalf("expected promo to become inactive after hitting max uses")
	}
}

func TestCreatePromoRejectsInvalidOrDuplicateCodes(t *testing.T) {
	store := newTestRegistryStore(t)

	invite, err := store.CreateInvite(InviteCreateRequest{Name: "invite", MaxUses: 1})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, err = store.CreatePromo(PromoCreateRequest{
		Code:       invite.Code,
		Name:       "conflict",
		BonusBytes: 1024,
	})
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected invite conflict error, got: %v", err)
	}

	_, err = store.CreatePromo(PromoCreateRequest{
		Code:       "first.code",
		Name:       "first",
		BonusBytes: 1024,
	})
	if err != nil {
		t.Fatalf("create first promo: %v", err)
	}

	_, err = store.CreatePromo(PromoCreateRequest{
		Code:       "FIRST.CODE",
		Name:       "duplicate",
		BonusBytes: 1024,
	})
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected duplicate promo conflict error, got: %v", err)
	}

	_, err = store.CreatePromo(PromoCreateRequest{
		Code:       "bad code!",
		Name:       "invalid",
		BonusBytes: 1024,
	})
	if err == nil || !strings.Contains(err.Error(), "supports only") {
		t.Fatalf("expected invalid code format error, got: %v", err)
	}
}

func TestPromoExpiresWhenTemporary(t *testing.T) {
	store := newTestRegistryStore(t)

	invite, err := store.CreateInvite(InviteCreateRequest{Name: "invite", MaxUses: 1})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if _, err := store.RedeemInvite(invite.Code, "temp-device", "temp-device"); err != nil {
		t.Fatalf("redeem invite: %v", err)
	}

	promo, err := store.CreatePromo(PromoCreateRequest{
		Code:         "temp-short",
		Name:         "temp",
		BonusBytes:   1024,
		ExpiresAfter: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create promo: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	_, err = store.RedeemPromo(promo.Code, "temp-device", "temp-device")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired promo error, got: %v", err)
	}
}

func TestRedeemPromoCreatesTrialClientWhenNoBoundInvite(t *testing.T) {
	store := newTestRegistryStore(t)

	promo, err := store.CreatePromo(PromoCreateRequest{
		Code:       "trial-2026",
		Name:       "Trial tariff",
		BonusBytes: 2 * 1024 * 1024,
		MaxUses:    10,
	})
	if err != nil {
		t.Fatalf("create promo: %v", err)
	}

	result, err := store.RedeemPromo(promo.Code, "new-device-1", "Pixel")
	if err != nil {
		t.Fatalf("redeem promo: %v", err)
	}
	if result.ActivationMode != PromoActivationModeTrial {
		t.Fatalf("expected trial activation mode, got %q", result.ActivationMode)
	}
	if result.Client.DeviceID != "new-device-1" {
		t.Fatalf("unexpected device id: %q", result.Client.DeviceID)
	}
	if result.Client.TrafficLimitBytes != promo.BonusBytes {
		t.Fatalf("expected trial limit %d, got %d", promo.BonusBytes, result.Client.TrafficLimitBytes)
	}
	if result.Client.TrafficBonusBytes != promo.BonusBytes {
		t.Fatalf("expected trial bonus %d, got %d", promo.BonusBytes, result.Client.TrafficBonusBytes)
	}
}

func TestRedeemPromoKeepsBonusFlowForBoundClient(t *testing.T) {
	store := newTestRegistryStore(t)

	invite, err := store.CreateInvite(InviteCreateRequest{
		Name:              "base",
		MaxUses:           1,
		TrafficLimitBytes: 3 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	baseResult, err := store.RedeemInvite(invite.Code, "bound-device-1", "Device")
	if err != nil {
		t.Fatalf("redeem invite: %v", err)
	}
	initialLimit := baseResult.Client.TrafficLimitBytes

	promo, err := store.CreatePromo(PromoCreateRequest{
		Code:       "bonus-2026",
		Name:       "Bonus",
		BonusBytes: 1024,
		MaxUses:    10,
	})
	if err != nil {
		t.Fatalf("create promo: %v", err)
	}

	result, err := store.RedeemPromo(promo.Code, "bound-device-1", "Device")
	if err != nil {
		t.Fatalf("redeem promo: %v", err)
	}
	if result.ActivationMode != PromoActivationModeBonus {
		t.Fatalf("expected bonus activation mode, got %q", result.ActivationMode)
	}
	if result.Client.TrafficLimitBytes != initialLimit+promo.BonusBytes {
		t.Fatalf("expected updated limit %d, got %d", initialLimit+promo.BonusBytes, result.Client.TrafficLimitBytes)
	}
}

func TestBuildClientProfilesForAdditionalServerUsesPrimaryPublicKeyFallback(t *testing.T) {
	cfg := config.RealityConfig{
		PublicHost: "2.26.85.47",
		PublicPort: 443,
		ServerNames: []string{
			"www.cloudflare.com",
		},
		AdditionalServers: []config.RealityAdditionalServerConfig{
			{
				Name:       "Switzerland (fast)",
				PublicHost: "87.121.105.190",
				PublicPort: 8443,
				ServerNames: []string{
					"www.cloudflare.com",
				},
				VPNOnly: true,
			},
		},
	}

	state := State{
		PublicKey: "primary-public-key",
		ShortIDs:  []string{"abcd1234"},
	}
	client := ClientRecord{
		Name: "test-device",
		UUID: "11111111-1111-1111-1111-111111111111",
	}

	profiles := buildClientProfilesFor(cfg, state, client)
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	additional := profiles[1]
	if additional.Address != "87.121.105.190" {
		t.Fatalf("unexpected additional profile address: %q", additional.Address)
	}
	if additional.PublicKey != state.PublicKey {
		t.Fatalf("expected fallback public key %q, got %q", state.PublicKey, additional.PublicKey)
	}
	if !strings.Contains(additional.Name, "(VPN)") {
		t.Fatalf("expected VPN suffix in profile name, got %q", additional.Name)
	}
}

func TestMergeRemoteIgnoresNonRuntimeDifferences(t *testing.T) {
	t.Parallel()

	store := NewRegistryStore(filepath.Join(t.TempDir(), "registry.json"), nil)
	createdAt := time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC)
	localTrafficSync := createdAt.Add(10 * time.Minute)
	bootstrapUpdatedAt := createdAt.Add(20 * time.Minute)

	local := Registry{
		Version:           2,
		BootstrapClientID: "bootstrap",
		Server: RegistryServer{
			PublicHost: "87.121.105.190",
			PublicPort: 8443,
		},
		Clients: []ClientRecord{
			{
				ID:         "bootstrap",
				Name:       "Bootstrap device",
				DeviceID:   "bootstrap",
				DeviceName: "Bootstrap device",
				UUID:       "bootstrap-uuid",
				Email:      "bootstrap@novpn",
				CreatedAt:  createdAt,
				UpdatedAt:  bootstrapUpdatedAt,
				Active:     true,
			},
			{
				ID:                "client-1",
				UUID:              "client-uuid",
				Email:             "client@novpn",
				CreatedAt:         createdAt.Add(time.Minute),
				Active:            true,
				TrafficUsedBytes:  1024,
				LastTrafficSyncAt: &localTrafficSync,
			},
		},
	}
	if _, err := store.Update(func(registry *Registry) error {
		*registry = local
		return nil
	}); err != nil {
		t.Fatalf("seed local registry: %v", err)
	}

	remote := local
	remote.Server.PublicHost = "2.26.85.47"
	remote.Clients[0].UpdatedAt = createdAt.Add(40 * time.Minute)
	remote.Clients[1].TrafficUsedBytes = 1
	remoteTrafficSync := createdAt.Add(50 * time.Minute)
	remote.Clients[1].LastTrafficSyncAt = &remoteTrafficSync

	changed, merged, err := store.MergeRemote(remote)
	if err != nil {
		t.Fatalf("merge remote registry: %v", err)
	}
	if changed {
		t.Fatalf("expected runtime comparison to ignore metadata-only diffs")
	}
	if merged.Server.PublicHost != local.Server.PublicHost {
		t.Fatalf("expected local VPN server host to be preserved, got %q", merged.Server.PublicHost)
	}
	bootstrap := merged.findClient("bootstrap")
	if bootstrap == nil {
		t.Fatalf("expected bootstrap client to remain present")
	}
	if !bootstrap.UpdatedAt.Equal(bootstrapUpdatedAt) {
		t.Fatalf("expected bootstrap updated_at to stay local, got %s", bootstrap.UpdatedAt)
	}
	client := merged.findClient("client-1")
	if client == nil {
		t.Fatalf("expected merged client to remain present")
	}
	if client.TrafficUsedBytes != 1024 {
		t.Fatalf("expected higher local traffic counter to be preserved, got %d", client.TrafficUsedBytes)
	}
}

func TestMergeRemoteDetectsRuntimeChanges(t *testing.T) {
	t.Parallel()

	store := NewRegistryStore(filepath.Join(t.TempDir(), "registry.json"), nil)
	createdAt := time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC)

	local := Registry{
		Version:           2,
		BootstrapClientID: "bootstrap",
		Clients: []ClientRecord{
			{
				ID:        "bootstrap",
				DeviceID:  "bootstrap",
				UUID:      "bootstrap-uuid",
				Email:     "bootstrap@novpn",
				CreatedAt: createdAt,
				Active:    true,
			},
			{
				ID:        "client-1",
				UUID:      "client-uuid",
				Email:     "client@novpn",
				CreatedAt: createdAt.Add(time.Minute),
				Active:    true,
			},
		},
	}
	if _, err := store.Update(func(registry *Registry) error {
		*registry = local
		return nil
	}); err != nil {
		t.Fatalf("seed local registry: %v", err)
	}

	remote := local
	remote.Clients[1].Email = "client-renamed@novpn"

	changed, _, err := store.MergeRemote(remote)
	if err != nil {
		t.Fatalf("merge remote registry: %v", err)
	}
	if !changed {
		t.Fatalf("expected runtime comparison to detect active client identity change")
	}
}

func newTestRegistryStore(t *testing.T) *RegistryStore {
	t.Helper()
	return NewRegistryStore(filepath.Join(t.TempDir(), "registry.json"), nil)
}
