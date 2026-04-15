package controlplane

import (
	"testing"
	"time"
)

func TestCatalogNormalizeDropsMissingServerReferences(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 15, 16, 0, 0, 0, time.UTC)
	snapshot := CatalogSnapshot{
		Servers: []ServerNode{
			{
				ID:         "primary",
				Name:       "Primary",
				Address:    "87.121.105.190",
				Port:       8443,
				ServerName: "www.cloudflare.com",
				Active:     true,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
			{
				ID:         "disabled",
				Name:       "Disabled",
				Address:    "10.0.0.2",
				Port:       8443,
				ServerName: "www.cloudflare.com",
				Active:     false,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		},
		Plans: []SubscriptionPlan{
			{
				ID:        "starter",
				Name:      "Starter",
				ServerIDs: []string{"primary", "missing-node", "disabled"},
				Active:    true,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "broken",
				Name:      "Broken",
				ServerIDs: []string{"ghost"},
				Active:    true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}

	snapshot.normalize()

	if got := snapshot.Plans[0].ServerIDs; len(got) != 1 || got[0] != "primary" {
		t.Fatalf("expected only existing active server to remain, got %#v", got)
	}
	if snapshot.Plans[0].Active != true {
		t.Fatalf("expected partially valid plan to stay active")
	}
	if len(snapshot.Plans[1].ServerIDs) != 0 {
		t.Fatalf("expected invalid server references to be removed, got %#v", snapshot.Plans[1].ServerIDs)
	}
	if snapshot.Plans[1].Active {
		t.Fatalf("expected plan without valid servers to be deactivated")
	}
}
