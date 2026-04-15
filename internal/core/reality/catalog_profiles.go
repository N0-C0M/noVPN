package reality

import (
	"fmt"
	"strings"
	"time"

	"novpn/internal/controlplane"
)

func BuildClientProfilesForCatalog(state State, client ClientRecord, servers []controlplane.ServerNode) []ClientProfile {
	allowed := normalizeAllowedServerSet(client.AllowedServerIDs)
	profiles := make([]ClientProfile, 0, len(servers))
	baseName := clientDisplayName(client)

	for index, server := range servers {
		if !server.Active || strings.TrimSpace(server.Address) == "" || server.Port <= 0 {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[strings.TrimSpace(strings.ToLower(server.ID))]; !ok {
				continue
			}
		}

		shortIDs := append([]string(nil), server.ShortIDs...)
		if len(shortIDs) == 0 {
			shortIDs = append(shortIDs, state.ShortIDs...)
		}
		shortID := strings.TrimSpace(server.ShortID)
		if shortID == "" && len(shortIDs) > 0 {
			shortID = strings.TrimSpace(shortIDs[0])
		}

		profileName := baseName
		if !server.Primary || index > 0 || len(servers) > 1 {
			profileName = fmt.Sprintf("%s · %s", baseName, server.Name)
		}

		profiles = append(profiles, ClientProfile{
			GeneratedAt: time.Now().UTC(),
			Name:        profileName,
			Type:        "vless-reality",
			ServerID:    server.ID,
			Address:     server.Address,
			Port:        server.Port,
			UUID:        client.UUID,
			Flow:        firstNonEmpty(strings.TrimSpace(server.Flow), "xtls-rprx-vision"),
			Network:     "tcp",
			Security:    "reality",
			ServerName:  strings.TrimSpace(server.ServerName),
			Fingerprint: firstNonEmpty(strings.TrimSpace(server.Fingerprint), "chrome"),
			PublicKey:   firstNonEmpty(strings.TrimSpace(server.PublicKey), strings.TrimSpace(state.PublicKey)),
			ShortID:     shortID,
			ShortIDs:    shortIDs,
			SpiderX:     firstNonEmpty(strings.TrimSpace(server.SpiderX), "/"),
			Location:    strings.TrimSpace(server.LocationLabel),
		})
	}

	return profiles
}

func clientDisplayName(client ClientRecord) string {
	name := strings.TrimSpace(client.Name)
	if name != "" {
		return name
	}
	name = strings.TrimSpace(client.DeviceName)
	if name != "" {
		return name
	}
	return "novpn-device"
}

func normalizeAllowedServerSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}
