package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type serverClientProfile struct {
	ServerID      string   `yaml:"server_id"`
	Name          string   `yaml:"name"`
	Address       string   `yaml:"address"`
	Port          int      `yaml:"port"`
	UUID          string   `yaml:"uuid"`
	Flow          string   `yaml:"flow"`
	ServerName    string   `yaml:"server_name"`
	Fingerprint   string   `yaml:"fingerprint"`
	PublicKey     string   `yaml:"public_key"`
	ShortID       string   `yaml:"short_id"`
	ShortIDs      []string `yaml:"short_ids"`
	SpiderX       string   `yaml:"spider_x"`
	LocationLabel string   `yaml:"location_label"`
	APIBase       string   `yaml:"api_base"`
}

type clientProfile struct {
	Name        string            `json:"name"`
	Server      clientServer      `json:"server"`
	Local       clientLocal       `json:"local"`
	Obfuscation clientObfuscation `json:"obfuscation"`
}

type clientServer struct {
	ServerID      string `json:"server_id,omitempty"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	UUID          string `json:"uuid"`
	Flow          string `json:"flow"`
	ServerName    string `json:"server_name"`
	Fingerprint   string `json:"fingerprint"`
	PublicKey     string `json:"public_key"`
	ShortID       string `json:"short_id"`
	LocationLabel string `json:"location_label,omitempty"`
	SpiderX       string `json:"spider_x"`
	APIBase       string `json:"api_base,omitempty"`
}

type clientLocal struct {
	SocksListen string `json:"socks_listen"`
	SocksPort   int    `json:"socks_port"`
	HTTPListen  string `json:"http_listen"`
	HTTPPort    int    `json:"http_port"`
}

type clientObfuscation struct {
	Seed string `json:"seed"`
}

type androidBootstrap struct {
	ServerAddress string `json:"server_address"`
	APIBase       string `json:"api_base,omitempty"`
}

func main() {
	var (
		inputPath     string
		commonOutput  string
		androidOutput string
		profileName   string
		seed          string
		bootstrapAddr string
		bootstrapAPI  string
	)

	flag.StringVar(&inputPath, "input", "", "path to server client-profile.yaml")
	flag.StringVar(&commonOutput, "common-output", "client/common/profiles/reality/default.profile.json", "path to common client profile JSON")
	flag.StringVar(&androidOutput, "android-output", "client/android/app/src/main/secure/bootstrap.json", "path to Android bootstrap JSON")
	flag.StringVar(&profileName, "name", "Default Reality Profile", "profile display name for generated JSON")
	flag.StringVar(&seed, "seed", "", "shared obfuscation seed override")
	flag.StringVar(&bootstrapAddr, "bootstrap-address", "", "override bootstrap server address used by Android before the first invite/profile sync")
	flag.StringVar(&bootstrapAPI, "bootstrap-api-base", "", "override control-plane API base URL stored in generated profiles and Android bootstrap")
	flag.Parse()

	if strings.TrimSpace(inputPath) == "" {
		exitf("missing required -input <path>")
	}

	profile, err := loadServerProfile(inputPath)
	if err != nil {
		exitf("%v", err)
	}

	if seed == "" {
		seed = defaultSeed(profile)
	}

	document := buildClientProfile(profile, profileName, seed, bootstrapAPI)
	if err := writeClientProfile(commonOutput, document); err != nil {
		exitf("%v", err)
	}
	fmt.Printf("updated %s\n", filepath.Clean(commonOutput))

	if strings.TrimSpace(bootstrapAddr) == "" {
		bootstrapAddr = profile.Address
	}
	if err := writeAndroidBootstrap(androidOutput, bootstrapAddr, bootstrapAPI); err != nil {
		exitf("%v", err)
	}
	fmt.Printf("updated %s\n", filepath.Clean(androidOutput))
}

func loadServerProfile(path string) (serverClientProfile, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return serverClientProfile{}, fmt.Errorf("read %s: %w", path, err)
	}

	var profile serverClientProfile
	if err := yaml.Unmarshal(payload, &profile); err != nil {
		return serverClientProfile{}, fmt.Errorf("decode %s: %w", path, err)
	}

	if profile.ShortID == "" && len(profile.ShortIDs) > 0 {
		profile.ShortID = strings.TrimSpace(profile.ShortIDs[0])
	}
	if profile.SpiderX == "" {
		profile.SpiderX = "/"
	}
	if err := validateServerProfile(profile); err != nil {
		return serverClientProfile{}, err
	}
	return profile, nil
}

func validateServerProfile(profile serverClientProfile) error {
	switch {
	case strings.TrimSpace(profile.Address) == "":
		return errors.New("client profile is missing address")
	case profile.Port <= 0:
		return errors.New("client profile is missing port")
	case strings.TrimSpace(profile.UUID) == "":
		return errors.New("client profile is missing uuid")
	case strings.TrimSpace(profile.PublicKey) == "":
		return errors.New("client profile is missing public_key")
	case strings.TrimSpace(profile.ShortID) == "":
		return errors.New("client profile is missing short_id")
	case strings.TrimSpace(profile.ServerName) == "":
		return errors.New("client profile is missing server_name")
	case strings.TrimSpace(profile.Fingerprint) == "":
		return errors.New("client profile is missing fingerprint")
	}
	return nil
}

func buildClientProfile(profile serverClientProfile, profileName string, seed string, bootstrapAPI string) clientProfile {
	apiBase := strings.TrimSpace(profile.APIBase)
	if apiBase == "" {
		apiBase = strings.TrimSpace(bootstrapAPI)
	}
	return clientProfile{
		Name: profileName,
		Server: clientServer{
			ServerID:      strings.TrimSpace(profile.ServerID),
			Address:       profile.Address,
			Port:          profile.Port,
			UUID:          profile.UUID,
			Flow:          profile.Flow,
			ServerName:    profile.ServerName,
			Fingerprint:   profile.Fingerprint,
			PublicKey:     profile.PublicKey,
			ShortID:       profile.ShortID,
			LocationLabel: strings.TrimSpace(profile.LocationLabel),
			SpiderX:       profile.SpiderX,
			APIBase:       apiBase,
		},
		Local: clientLocal{
			SocksListen: "127.0.0.1",
			SocksPort:   10808,
			HTTPListen:  "127.0.0.1",
			HTTPPort:    10809,
		},
		Obfuscation: clientObfuscation{
			Seed: seed,
		},
	}
}

func writeClientProfile(path string, profile clientProfile) error {
	payload, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeAndroidBootstrap(path string, serverAddress string, apiBase string) error {
	payload, err := json.MarshalIndent(androidBootstrap{
		ServerAddress: strings.TrimSpace(serverAddress),
		APIBase:       strings.TrimSpace(apiBase),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func defaultSeed(profile serverClientProfile) string {
	base := strings.TrimSpace(profile.ShortID)
	if base == "" {
		base = "novpn-default"
	}
	return "novpn-seed-" + base
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
