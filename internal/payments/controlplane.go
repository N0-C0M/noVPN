package payments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ControlPlaneClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type ActivateResponse struct {
	Invite struct {
		Code string `json:"code"`
	} `json:"invite"`
	RedeemURL    string `json:"redeem_url"`
	APIRedeemURL string `json:"api_redeem_url"`
	PublicAPI    string `json:"public_api"`
}

type RedeemResponse struct {
	Client struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
	} `json:"client"`
	ClientProfileVLESSURL  string   `json:"client_profile_vless_url"`
	ClientProfilesVLESSURL []string `json:"client_profiles_vless_urls"`
	SubscriptionURL        string   `json:"subscription_url"`
	SubscriptionText       string   `json:"subscription_text"`
	ClientProfilesYAML     []string `json:"client_profiles_yaml"`
	ClientProfileYAML      string   `json:"client_profile_yaml"`
}

func NewControlPlaneClient(baseURL string, token string) *ControlPlaneClient {
	return &ControlPlaneClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *ControlPlaneClient) Activate(planID string, name string, note string, maxUses int) (ActivateResponse, error) {
	payload := map[string]any{
		"plan_id":  strings.TrimSpace(planID),
		"name":     strings.TrimSpace(name),
		"note":     strings.TrimSpace(note),
		"max_uses": maxUses,
	}
	var response ActivateResponse
	err := c.postJSON(c.baseURL+"/control-plane/payments/activate", payload, true, &response)
	return response, err
}

func (c *ControlPlaneClient) Redeem(inviteCode string, deviceID string, deviceName string) (RedeemResponse, error) {
	payload := map[string]any{
		"device_id":   strings.TrimSpace(deviceID),
		"device_name": strings.TrimSpace(deviceName),
	}
	var response RedeemResponse
	endpoint := c.baseURL + "/redeem/" + url.PathEscape(strings.TrimSpace(inviteCode))
	if err := c.postJSON(endpoint, payload, false, &response); err != nil {
		return RedeemResponse{}, err
	}
	if len(response.ClientProfilesYAML) == 0 && strings.TrimSpace(response.ClientProfileYAML) != "" {
		response.ClientProfilesYAML = []string{strings.TrimSpace(response.ClientProfileYAML)}
	}
	return response, nil
}

func (c *ControlPlaneClient) Reject(clientID string, inviteCode string, reason string, source string) error {
	payload := map[string]any{
		"client_id":   strings.TrimSpace(clientID),
		"invite_code": strings.TrimSpace(inviteCode),
		"reason":      strings.TrimSpace(reason),
		"source":      strings.TrimSpace(source),
	}
	return c.postJSON(c.baseURL+"/control-plane/payments/reject", payload, true, nil)
}

func (c *ControlPlaneClient) postJSON(endpoint string, payload any, withToken bool, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	if withToken && c.token != "" {
		req.Header.Set("X-Control-Plane-Token", c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("control plane %s: %s", endpoint, message)
	}
	if target == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode control plane response: %w", err)
	}
	return nil
}
