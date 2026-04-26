package server

import (
	"fmt"
	"net/http"
	"strings"

	"novpn/internal/core/reality"
)

func happSubscriptionObservationFromRequest(r *http.Request) (reality.SubscriptionDeviceObservation, bool) {
	deviceID := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-HWID"),
		r.URL.Query().Get("hwid"),
	))
	if deviceID == "" {
		return reality.SubscriptionDeviceObservation{}, false
	}

	deviceOS := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Device-OS"),
		r.URL.Query().Get("device_os"),
	))
	deviceOSVersion := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Ver-OS"),
		r.URL.Query().Get("device_os_version"),
	))
	deviceModel := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Device-Model"),
		r.URL.Query().Get("device_model"),
	))
	userAgent := strings.TrimSpace(r.UserAgent())

	return reality.SubscriptionDeviceObservation{
		DeviceID:        deviceID,
		DeviceName:      happObservedDeviceName(deviceModel, deviceOS, deviceOSVersion),
		DeviceOS:        deviceOS,
		DeviceOSVersion: deviceOSVersion,
		UserAgent:       userAgent,
	}, true
}

func happObservedDeviceName(deviceModel string, deviceOS string, deviceOSVersion string) string {
	deviceModel = strings.TrimSpace(deviceModel)
	deviceOS = strings.TrimSpace(deviceOS)
	deviceOSVersion = strings.TrimSpace(deviceOSVersion)

	switch {
	case deviceModel != "" && deviceOS != "" && deviceOSVersion != "":
		return fmt.Sprintf("%s (%s %s)", deviceModel, deviceOS, deviceOSVersion)
	case deviceModel != "" && deviceOS != "":
		return fmt.Sprintf("%s (%s)", deviceModel, deviceOS)
	case deviceModel != "":
		return deviceModel
	case deviceOS != "" && deviceOSVersion != "":
		return fmt.Sprintf("%s %s", deviceOS, deviceOSVersion)
	default:
		return deviceOS
	}
}

func formatObservedDevices(records []reality.ObservedDeviceRecord) string {
	if len(records) == 0 {
		return ""
	}

	parts := make([]string, 0, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.DeviceName)
		if name == "" {
			name = strings.TrimSpace(record.DeviceID)
		}
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "; ")
}
