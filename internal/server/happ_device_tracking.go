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
		r.Header.Get("X-Device-ID"),
		r.Header.Get("X-Client-Device-ID"),
		r.URL.Query().Get("hwid"),
		r.URL.Query().Get("device_id"),
	))
	if deviceID == "" {
		return reality.SubscriptionDeviceObservation{}, false
	}

	deviceName := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Device-Name"),
		r.Header.Get("X-Client-Device-Name"),
		r.URL.Query().Get("device_name"),
	))
	deviceOS := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Device-OS"),
		r.Header.Get("X-Client-Device-OS"),
		r.URL.Query().Get("device_os"),
	))
	deviceOSVersion := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Ver-OS"),
		r.Header.Get("X-Device-OS-Version"),
		r.Header.Get("X-Client-Device-OS-Version"),
		r.URL.Query().Get("device_os_version"),
	))
	deviceModel := strings.TrimSpace(firstNonEmptyString(
		r.Header.Get("X-Device-Model"),
		r.Header.Get("X-Client-Device-Model"),
		r.URL.Query().Get("device_model"),
	))
	userAgent := strings.TrimSpace(r.UserAgent())
	if deviceName == "" {
		deviceName = happObservedDeviceName(deviceModel, deviceOS, deviceOSVersion)
	}
	if deviceName == "" {
		deviceName = deviceID
	}

	return reality.SubscriptionDeviceObservation{
		DeviceID:        deviceID,
		DeviceName:      deviceName,
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
