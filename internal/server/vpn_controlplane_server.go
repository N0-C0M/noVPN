package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"novpn/internal/config"
)

func newVPNControlPlaneServer(cfg config.ControlPlaneConfig, gateway *Gateway, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/control-plane/system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !vpnControlPlaneAuthorized(r, cfg.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		snapshot, err := collectSystemStatusSnapshot("/", gateway.Ready())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snapshot); err != nil {
			if logger != nil {
				logger.Warn("encode vpn control-plane system snapshot failed", "error", err)
			}
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})

	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}
}

func vpnControlPlaneAuthorized(r *http.Request, token string) bool {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return false
	}
	return strings.TrimSpace(r.Header.Get("X-Control-Plane-Token")) == normalizedToken
}
