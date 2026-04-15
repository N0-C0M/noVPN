package server

import (
	"testing"
	"time"

	"novpn/internal/controlplane"
	"novpn/internal/core/reality"
)

func TestCatalogProfilesForClientDoesNotFallbackWhenAllowedServersAreMissing(t *testing.T) {
	t.Parallel()

	profiles, authoritative := catalogProfilesForClient(
		reality.State{},
		reality.ClientRecord{
			UUID:             "11111111-1111-1111-1111-111111111111",
			AllowedServerIDs: []string{"missing"},
		},
		nil,
	)
	if !authoritative {
		t.Fatalf("expected missing allowed servers to remain authoritative")
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles for unknown allowed servers, got %d", len(profiles))
	}
}

func TestCatalogProfilesForClientFallsBackOnlyWhenNoRestrictionsExist(t *testing.T) {
	t.Parallel()

	_, authoritative := catalogProfilesForClient(
		reality.State{},
		reality.ClientRecord{
			UUID: "11111111-1111-1111-1111-111111111111",
		},
		nil,
	)
	if authoritative {
		t.Fatalf("expected unrestricted client to allow fallback")
	}
}

func TestCatalogProfilesForClientBuildsProfilesForValidServers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 15, 16, 0, 0, 0, time.UTC)
	profiles, authoritative := catalogProfilesForClient(
		reality.State{
			PublicKey: "pub",
			ShortIDs:  []string{"abcd1234"},
		},
		reality.ClientRecord{
			Name:             "Phone",
			UUID:             "11111111-1111-1111-1111-111111111111",
			AllowedServerIDs: []string{"primary"},
			CreatedAt:        now,
		},
		[]controlplane.ServerNode{
			{
				ID:            "primary",
				Name:          "Primary",
				Address:       "87.121.105.190",
				Port:          8443,
				Flow:          "xtls-rprx-vision",
				ServerName:    "www.cloudflare.com",
				Fingerprint:   "chrome",
				Active:        true,
				Primary:       true,
				LocationLabel: "Sweden",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	)
	if !authoritative {
		t.Fatalf("expected valid catalog profiles to be authoritative")
	}
	if len(profiles) != 1 {
		t.Fatalf("expected one catalog profile, got %d", len(profiles))
	}
	if profiles[0].ServerID != "primary" {
		t.Fatalf("unexpected server id %q", profiles[0].ServerID)
	}
}
