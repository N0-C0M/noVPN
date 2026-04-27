package server

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"novpn/internal/core/reality"
)

var errClientNotFound = errors.New("client not found")

func marshalClientProfileVLESSURL(profile reality.ClientProfile) (string, error) {
	address := strings.TrimSpace(profile.Address)
	if address == "" {
		return "", errors.New("client profile is missing address")
	}
	if profile.Port <= 0 {
		return "", errors.New("client profile is missing port")
	}
	uuid := strings.TrimSpace(profile.UUID)
	if uuid == "" {
		return "", errors.New("client profile is missing uuid")
	}

	query := url.Values{}
	query.Set("encryption", "none")
	query.Set("security", firstNonEmptyString(strings.TrimSpace(profile.Security), "reality"))
	query.Set("type", firstNonEmptyString(strings.TrimSpace(profile.Network), "tcp"))

	if flow := strings.TrimSpace(profile.Flow); flow != "" {
		query.Set("flow", flow)
	}
	if serverName := strings.TrimSpace(profile.ServerName); serverName != "" {
		query.Set("sni", serverName)
	}
	if fingerprint := strings.TrimSpace(profile.Fingerprint); fingerprint != "" {
		query.Set("fp", fingerprint)
	}
	if publicKey := strings.TrimSpace(profile.PublicKey); publicKey != "" {
		query.Set("pbk", publicKey)
	}
	if shortID := strings.TrimSpace(profile.ShortID); shortID != "" {
		query.Set("sid", shortID)
	}
	if spiderX := strings.TrimSpace(profile.SpiderX); spiderX != "" {
		query.Set("spx", spiderX)
	}

	title := strings.TrimSpace(profile.Name)
	if title == "" {
		title = strings.TrimSpace(profile.Location)
	}

	link := url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     net.JoinHostPort(address, strconv.Itoa(profile.Port)),
		RawQuery: query.Encode(),
		Fragment: title,
	}
	return link.String(), nil
}

func marshalClientProfileVLESSURLList(profiles []reality.ClientProfile) ([]string, error) {
	result := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		link, err := marshalClientProfileVLESSURL(profile)
		if err != nil {
			return nil, err
		}
		result = append(result, link)
	}
	return result, nil
}

func marshalClientProfileSubscriptionText(profiles []reality.ClientProfile) (string, error) {
	links, err := marshalClientProfileVLESSURLList(profiles)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", nil
	}
	return strings.Join(links, "\n") + "\n", nil
}

func (a *adminApp) addClientProfileLinkExports(payload map[string]any, client reality.ClientRecord, profiles []reality.ClientProfile) error {
	if len(profiles) == 0 {
		return nil
	}

	link, err := marshalClientProfileVLESSURL(profiles[0])
	if err != nil {
		return err
	}
	links, err := marshalClientProfileVLESSURLList(profiles)
	if err != nil {
		return err
	}
	subscriptionText, err := marshalClientProfileSubscriptionText(profiles)
	if err != nil {
		return err
	}

	payload["client_profile_vless_url"] = link
	payload["client_profiles_vless_urls"] = links
	payload["subscription_text"] = subscriptionText

	if subscriptionURL := a.clientSubscriptionURL(client.UUID); subscriptionURL != "" {
		payload["subscription_url"] = subscriptionURL
		payload["happ_subscription_url"] = subscriptionURL
	}
	return nil
}

func (a *adminApp) clientSubscriptionURL(clientUUID string) string {
	base := a.publicBaseURL()
	clientUUID = strings.TrimSpace(clientUUID)
	if base == "" || clientUUID == "" {
		return ""
	}
	shortBase := strings.TrimRight(strings.TrimSuffix(base, a.basePath), "/")
	if shortBase != "" {
		return shortBase + "/s/" + url.PathEscape(clientUUID)
	}
	return base + "/client/subscription?client_uuid=" + url.QueryEscape(clientUUID)
}

func (a *adminApp) resolveClientProfilesByID(clientID string) (reality.ClientRecord, []reality.ClientProfile, error) {
	return a.resolveClientProfiles(clientID, "", "", true)
}

func (a *adminApp) resolveClientProfilesBySelectors(clientUUID string, deviceID string) (reality.ClientRecord, []reality.ClientProfile, error) {
	return a.resolveClientProfiles("", clientUUID, deviceID, false)
}

func (a *adminApp) resolveClientProfiles(clientID string, clientUUID string, deviceID string, byID bool) (reality.ClientRecord, []reality.ClientProfile, error) {
	clients, err := a.reality.ListClients()
	if err != nil {
		return reality.ClientRecord{}, nil, err
	}

	var target reality.ClientRecord
	found := false
	for _, client := range clients {
		if byID {
			if client.ID != clientID {
				continue
			}
		} else {
			if !clientMatchesSelectors(client, clientUUID, deviceID) {
				continue
			}
		}
		target = client
		found = true
		break
	}
	if !found {
		return reality.ClientRecord{}, nil, errClientNotFound
	}

	state, err := a.reality.LoadState()
	if err != nil {
		return reality.ClientRecord{}, nil, err
	}
	clientProfiles := a.buildClientProfiles(state, target)
	if len(clientProfiles) == 0 {
		return reality.ClientRecord{}, nil, fmt.Errorf("server did not build client profiles")
	}
	return target, clientProfiles, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func clientMatchesSelectors(client reality.ClientRecord, clientUUID string, deviceID string) bool {
	clientUUID = strings.TrimSpace(clientUUID)
	deviceID = strings.TrimSpace(deviceID)
	if clientUUID == "" && deviceID == "" {
		return false
	}
	if clientUUID != "" && client.UUID != clientUUID {
		return false
	}
	if deviceID != "" && !clientHasDevice(client, deviceID) {
		return false
	}
	return true
}

func clientHasDevice(client reality.ClientRecord, deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	if client.DeviceID == deviceID {
		return true
	}
	for _, observed := range client.ObservedDevices {
		if observed.DeviceID == deviceID {
			return true
		}
	}
	return false
}
