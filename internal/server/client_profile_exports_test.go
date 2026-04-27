package server

import (
	"net/url"
	"strings"
	"testing"

	"novpn/internal/config"
	"novpn/internal/core/reality"
)

func TestMarshalClientProfileVLESSURLIncludesRealityParameters(t *testing.T) {
	t.Parallel()

	link, err := marshalClientProfileVLESSURL(reality.ClientProfile{
		Name:        "Alice iPhone",
		Address:     "vpn.example.com",
		Port:        443,
		UUID:        "11111111-1111-1111-1111-111111111111",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
		ServerName:  "cdn.example.com",
		Fingerprint: "chrome",
		PublicKey:   "pub-key",
		ShortID:     "abcd1234",
		SpiderX:     "/",
	})
	if err != nil {
		t.Fatalf("marshalClientProfileVLESSURL returned error: %v", err)
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	if parsed.Scheme != "vless" {
		t.Fatalf("unexpected scheme %q", parsed.Scheme)
	}
	if parsed.User == nil || parsed.User.Username() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected user info in %q", link)
	}
	if parsed.Host != "vpn.example.com:443" {
		t.Fatalf("unexpected host %q", parsed.Host)
	}
	if parsed.Fragment != "Alice iPhone" {
		t.Fatalf("unexpected fragment %q", parsed.Fragment)
	}

	values := parsed.Query()
	for key, want := range map[string]string{
		"encryption": "none",
		"security":   "reality",
		"type":       "tcp",
		"flow":       "xtls-rprx-vision",
		"sni":        "cdn.example.com",
		"fp":         "chrome",
		"pbk":        "pub-key",
		"sid":        "abcd1234",
		"spx":        "/",
	} {
		if got := values.Get(key); got != want {
			t.Fatalf("unexpected %s: got %q want %q", key, got, want)
		}
	}
}

func TestMarshalClientProfileSubscriptionTextUsesOneLinkPerLine(t *testing.T) {
	t.Parallel()

	text, err := marshalClientProfileSubscriptionText([]reality.ClientProfile{
		{
			Name:        "Phone",
			Address:     "1.1.1.1",
			Port:        443,
			UUID:        "11111111-1111-1111-1111-111111111111",
			Security:    "reality",
			ServerName:  "a.example.com",
			Fingerprint: "chrome",
			PublicKey:   "pub-a",
			ShortID:     "short-a",
			SpiderX:     "/",
		},
		{
			Name:        "Laptop",
			Address:     "2.2.2.2",
			Port:        8443,
			UUID:        "22222222-2222-2222-2222-222222222222",
			Security:    "reality",
			ServerName:  "b.example.com",
			Fingerprint: "firefox",
			PublicKey:   "pub-b",
			ShortID:     "short-b",
			SpiderX:     "/app",
		},
	})
	if err != nil {
		t.Fatalf("marshalClientProfileSubscriptionText returned error: %v", err)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("subscription text must end with a newline, got %q", text)
	}

	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), text)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "vless://") {
			t.Fatalf("expected vless link, got %q", line)
		}
	}
}

func TestClientSubscriptionURLUsesPublicBaseURL(t *testing.T) {
	t.Parallel()

	app := &adminApp{
		cfg: config.AdminConfig{
			PublicBaseURL: "https://panel.example.com",
		},
		basePath: "/admin",
	}

	got := app.clientSubscriptionURL("11111111-1111-1111-1111-111111111111")
	want := "https://panel.example.com/admin/client/subscription?client_uuid=11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Fatalf("unexpected subscription URL: got %q want %q", got, want)
	}
}

func TestClientMatchesSelectorsIncludesObservedDevices(t *testing.T) {
	t.Parallel()

	client := reality.ClientRecord{
		UUID:     "uuid-1",
		DeviceID: "desktop-1",
		ObservedDevices: []reality.ObservedDeviceRecord{
			{DeviceID: "android-1", DeviceName: "Pixel 9"},
		},
	}

	if !clientMatchesSelectors(client, "uuid-1", "android-1") {
		t.Fatalf("expected observed device to match client selectors")
	}
	if !clientMatchesSelectors(client, "", "android-1") {
		t.Fatalf("expected observed device to match without client uuid")
	}
	if clientMatchesSelectors(client, "uuid-2", "android-1") {
		t.Fatalf("expected mismatched client uuid to fail selector match")
	}
}
