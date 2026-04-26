package server

import (
	"net/http/httptest"
	"testing"

	"novpn/internal/core/reality"
)

func TestHappSubscriptionObservationFromRequestReadsOfficialHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "https://panel.example.com/admin/client/subscription?client_uuid=uuid-1", nil)
	req.Header.Set("X-HWID", "ios-hwid-1")
	req.Header.Set("X-Device-OS", "iOS")
	req.Header.Set("X-Ver-OS", "18.3")
	req.Header.Set("X-Device-Model", "iPhone 15")
	req.Header.Set("User-Agent", "Happ/3.13.0")

	observation, ok := happSubscriptionObservationFromRequest(req)
	if !ok {
		t.Fatalf("expected Happ observation to be detected")
	}
	if observation.DeviceID != "ios-hwid-1" {
		t.Fatalf("unexpected device id %q", observation.DeviceID)
	}
	if observation.DeviceName != "iPhone 15 (iOS 18.3)" {
		t.Fatalf("unexpected device name %q", observation.DeviceName)
	}
	if observation.DeviceOS != "iOS" {
		t.Fatalf("unexpected device os %q", observation.DeviceOS)
	}
	if observation.DeviceOSVersion != "18.3" {
		t.Fatalf("unexpected device os version %q", observation.DeviceOSVersion)
	}
	if observation.UserAgent != "Happ/3.13.0" {
		t.Fatalf("unexpected user agent %q", observation.UserAgent)
	}
}

func TestFormatObservedDevicesListsNames(t *testing.T) {
	t.Parallel()

	got := formatObservedDevices([]reality.ObservedDeviceRecord{
		{DeviceName: "iPhone 15 (iOS 18.3)", DeviceID: "ios-hwid-1"},
		{DeviceName: "", DeviceID: "android-hwid-2"},
	})
	if got != "iPhone 15 (iOS 18.3); android-hwid-2" {
		t.Fatalf("unexpected observed device list %q", got)
	}
}
