package reality

import (
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

func newTestRegistryStore(t *testing.T) *RegistryStore {
	t.Helper()
	return NewRegistryStore(filepath.Join(t.TempDir(), "registry.json"), nil)
}
