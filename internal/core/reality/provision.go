package reality

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"novpn/internal/config"
)

type Provisioner struct {
	cfg          config.RealityConfig
	logger       *slog.Logger
	registryStore *RegistryStore
}

type Options struct {
	InstallXray    bool
	ValidateConfig bool
	ManageService  bool
}

type Result struct {
	ConfigPath        string
	StatePath         string
	RegistryPath      string
	ClientProfilePath string
	State             State
	ClientProfile     ClientProfile
}

type State struct {
	CreatedAt  time.Time `yaml:"created_at,omitempty"`
	UpdatedAt  time.Time `yaml:"updated_at"`
	UUID       string    `yaml:"uuid"`
	PrivateKey string    `yaml:"private_key"`
	PublicKey  string    `yaml:"public_key"`
	ShortIDs   []string  `yaml:"short_ids"`
}

type ClientProfile struct {
	GeneratedAt time.Time `yaml:"generated_at"`
	Name        string    `yaml:"name"`
	Type        string    `yaml:"type"`
	Address     string    `yaml:"address"`
	Port        int       `yaml:"port"`
	UUID        string    `yaml:"uuid"`
	Flow        string    `yaml:"flow"`
	Network     string    `yaml:"network"`
	Security    string    `yaml:"security"`
	ServerName  string    `yaml:"server_name"`
	Fingerprint string    `yaml:"fingerprint"`
	PublicKey   string    `yaml:"public_key"`
	ShortID     string    `yaml:"short_id"`
	ShortIDs    []string  `yaml:"short_ids"`
	SpiderX     string    `yaml:"spider_x,omitempty"`
}

func NewProvisioner(cfg config.RealityConfig, logger *slog.Logger) *Provisioner {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Provisioner{
		cfg:           cfg,
		logger:        logger.With("component", "reality-bootstrap"),
		registryStore: NewRegistryStore(cfg.Xray.RegistryPath, logger),
	}
}

func (p *Provisioner) Bootstrap(ctx context.Context, opts Options) (Result, error) {
	if !p.cfg.Enabled {
		return Result{}, errors.New("core.reality is disabled")
	}

	state, err := p.ensureState()
	if err != nil {
		return Result{}, err
	}

	registry, err := p.registryStore.EnsureBootstrap(state, p.cfg)
	if err != nil {
		return Result{}, err
	}

	if opts.InstallXray {
		if err := p.installXray(ctx); err != nil {
			return Result{}, err
		}
	}

	if err := p.ensureParentDirs(); err != nil {
		return Result{}, err
	}

	configPayload, err := p.renderXrayConfig(state, registry)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileAtomically(p.cfg.Xray.ConfigPath, configPayload, 0o644); err != nil {
		return Result{}, fmt.Errorf("write xray config: %w", err)
	}

	profile, err := registry.ActiveClientProfile(state, p.cfg)
	if err != nil {
		return Result{}, err
	}
	profilePayload, err := yaml.Marshal(profile)
	if err != nil {
		return Result{}, fmt.Errorf("marshal client profile: %w", err)
	}
	if err := writeFileAtomically(p.cfg.Xray.ClientProfilePath, profilePayload, 0o600); err != nil {
		return Result{}, fmt.Errorf("write client profile: %w", err)
	}

	if opts.ValidateConfig {
		if err := p.validateConfig(ctx); err != nil {
			return Result{}, err
		}
	}

	if opts.ManageService {
		if err := p.restartService(ctx); err != nil {
			return Result{}, err
		}
	}

	return Result{
		ConfigPath:        p.cfg.Xray.ConfigPath,
		StatePath:         p.cfg.Xray.StatePath,
		RegistryPath:      p.cfg.Xray.RegistryPath,
		ClientProfilePath: p.cfg.Xray.ClientProfilePath,
		State:             state,
		ClientProfile:     profile,
	}, nil
}

func (p *Provisioner) ensureState() (State, error) {
	state, err := p.loadState()
	if err != nil {
		return State{}, err
	}

	now := time.Now().UTC()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}

	if p.cfg.UUID != "" {
		state.UUID = p.cfg.UUID
	}
	if p.cfg.PrivateKey != "" {
		state.PrivateKey = p.cfg.PrivateKey
	}
	if len(p.cfg.ShortIDs) > 0 {
		state.ShortIDs = append([]string(nil), p.cfg.ShortIDs...)
	}

	if state.UUID == "" {
		value, err := generateUUID()
		if err != nil {
			return State{}, err
		}
		state.UUID = value
	}

	if state.PrivateKey == "" {
		privateKey, publicKey, err := generateX25519KeyPair()
		if err != nil {
			return State{}, err
		}
		state.PrivateKey = privateKey
		state.PublicKey = publicKey
	} else {
		publicKey, err := derivePublicKey(state.PrivateKey)
		if err != nil {
			return State{}, fmt.Errorf("derive reality public key: %w", err)
		}
		state.PublicKey = publicKey
	}

	if len(state.ShortIDs) == 0 {
		shortID, err := generateShortID()
		if err != nil {
			return State{}, err
		}
		state.ShortIDs = []string{shortID}
	}

	state.UpdatedAt = now

	payload, err := yaml.Marshal(state)
	if err != nil {
		return State{}, fmt.Errorf("marshal reality state: %w", err)
	}
	if err := writeFileAtomically(p.cfg.Xray.StatePath, payload, 0o600); err != nil {
		return State{}, fmt.Errorf("write reality state: %w", err)
	}

	return state, nil
}

func (p *Provisioner) loadState() (State, error) {
	payload, err := os.ReadFile(p.cfg.Xray.StatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("read reality state: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(payload, &state); err != nil {
		return State{}, fmt.Errorf("unmarshal reality state: %w", err)
	}
	return state, nil
}

func (p *Provisioner) renderXrayConfig(state State, registry Registry) ([]byte, error) {
	document := map[string]any{
		"log": map[string]any{
			"loglevel": p.cfg.Xray.Log.Level,
			"access":   p.cfg.Xray.Log.AccessPath,
			"error":    p.cfg.Xray.Log.ErrorPath,
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "vless-reality-in",
				"listen":   listenHost(p.cfg.ListenAddr),
				"port":     listenPort(p.cfg.ListenAddr),
				"protocol": "vless",
				"settings": map[string]any{
					"decryption": "none",
					"clients":   registry.ActiveXrayClients(p.cfg.Flow),
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"show":         p.cfg.Show,
						"target":       p.cfg.Target,
						"xver":         p.cfg.Xver,
						"serverNames":  p.cfg.ServerNames,
						"privateKey":   state.PrivateKey,
						"shortIds":     state.ShortIDs,
						"minClientVer": p.cfg.MinClientVer,
						"maxClientVer": p.cfg.MaxClientVer,
						"maxTimeDiff":  p.cfg.MaxTimeDiffMillis,
					},
				},
				"sniffing": map[string]any{
					"enabled":      p.cfg.Sniffing.Enabled,
					"destOverride": p.cfg.Sniffing.DestOverride,
				},
			},
		},
		"outbounds": []any{
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
			},
			map[string]any{
				"tag":      "block",
				"protocol": "blackhole",
			},
		},
	}

	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray config: %w", err)
	}

	return append(payload, '\n'), nil
}

func (p *Provisioner) ensureParentDirs() error {
	paths := []string{
		p.cfg.Xray.ConfigPath,
		p.cfg.Xray.StatePath,
		p.cfg.Xray.ClientProfilePath,
		p.cfg.Xray.Log.AccessPath,
		p.cfg.Xray.Log.ErrorPath,
	}

	for _, path := range paths {
		dir := filepath.Dir(path)
		if dir == "." || dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	return nil
}

func (p *Provisioner) installXray(ctx context.Context) error {
	if strings.EqualFold(strings.TrimSpace(p.cfg.Xray.Install.Method), "none") {
		p.logger.Info("skipping Xray installation because install.method=none")
		return nil
	}
	if runtime.GOOS != "linux" {
		return errors.New("automatic Xray installation is supported only on Linux")
	}
	if !runningAsRoot() {
		return errors.New("automatic Xray installation requires root privileges")
	}

	scriptPath, err := downloadInstaller(ctx, p.cfg.Xray.Install.ScriptURL)
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)

	cmd := exec.CommandContext(ctx, "bash", scriptPath, "install", "--without-geodata")
	cmd.Env = append(os.Environ(), "JSON_PATH="+p.cfg.ConfigDir())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install Xray via official script: %w: %s", err, strings.TrimSpace(string(output)))
	}

	p.logger.Info("xray installation finished", "config_dir", p.cfg.ConfigDir())
	return nil
}

func (p *Provisioner) validateConfig(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return errors.New("xray config validation is supported only on Linux")
	}

	cmd := exec.CommandContext(ctx, p.cfg.Xray.BinaryPath, "run", "-test", "-config", p.cfg.Xray.ConfigPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate xray config: %w: %s", err, strings.TrimSpace(string(output)))
	}

	p.logger.Info("xray config validated", "config", p.cfg.Xray.ConfigPath)
	return nil
}

func (p *Provisioner) restartService(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return errors.New("service management is supported only on Linux")
	}
	if !runningAsRoot() {
		return errors.New("service management requires root privileges")
	}

	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", p.cfg.Xray.ServiceName},
		{"restart", p.cfg.Xray.ServiceName},
	} {
		cmd := exec.CommandContext(ctx, "systemctl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
	}

	p.logger.Info("xray service restarted", "service", p.cfg.Xray.ServiceName)
	return nil
}

func generateUUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}

	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(buffer[0:4]),
		hex.EncodeToString(buffer[4:6]),
		hex.EncodeToString(buffer[6:8]),
		hex.EncodeToString(buffer[8:10]),
		hex.EncodeToString(buffer[10:16]),
	), nil
}

func generateX25519KeyPair() (string, string, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate x25519 private key: %w", err)
	}

	privateEncoded := base64.RawURLEncoding.EncodeToString(privateKey.Bytes())
	publicEncoded := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	return privateEncoded, publicEncoded, nil
}

func derivePublicKey(privateKey string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return "", fmt.Errorf("decode base64 private key: %w", err)
	}

	parsed, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("parse x25519 private key: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(parsed.PublicKey().Bytes()), nil
}

func generateShortID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate short ID: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func downloadInstaller(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build installer request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download Xray installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download Xray installer: unexpected status %s", resp.Status)
	}

	file, err := os.CreateTemp("", "xray-install-*.sh")
	if err != nil {
		return "", fmt.Errorf("create temp installer file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("write temp installer file: %w", err)
	}

	if err := file.Chmod(0o700); err != nil {
		return "", fmt.Errorf("chmod temp installer file: %w", err)
	}

	return file.Name(), nil
}

func writeFileAtomically(path string, payload []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := file.Write(payload); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

func listenHost(addr string) string {
	host, _, err := splitAddr(addr)
	if err != nil {
		return ""
	}
	return host
}

func listenPort(addr string) int {
	_, port, err := splitAddr(addr)
	if err != nil {
		return 0
	}
	return port
}

func splitAddr(addr string) (string, int, error) {
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}

	var port int
	if _, err := fmt.Sscanf(rawPort, "%d", &port); err != nil {
		return "", 0, fmt.Errorf("parse port %q: %w", rawPort, err)
	}
	return host, port, nil
}

func runningAsRoot() bool {
	current, err := user.Current()
	if err != nil {
		return false
	}
	return current.Username == "root"
}
