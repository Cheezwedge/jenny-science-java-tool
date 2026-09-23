package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// Settings are remembered between runs so the program can simply be
// double-clicked. On Windows they live in %APPDATA%\xenax-bridge\settings.json.
type Settings struct {
	Drive     string `json:"drive"`
	AsciiPort int    `json:"asciiPort"`
	HTTPPort  int    `json:"httpPort"`
}

func defaultSettings() Settings {
	return Settings{Drive: "192.168.2.100", AsciiPort: 10001, HTTPPort: 80}
}

func (s Settings) Validate() error {
	host := strings.TrimSpace(s.Drive)
	if host == "" || strings.ContainsAny(host, " /\\:?#@") {
		return fmt.Errorf("invalid drive address %q", s.Drive)
	}
	if s.AsciiPort < 1 || s.AsciiPort > 65535 || s.HTTPPort < 1 || s.HTTPPort > 65535 {
		return errors.New("ports must be between 1 and 65535")
	}
	return nil
}

func (s Settings) ASCIIAddr() string { return net.JoinHostPort(s.Drive, fmt.Sprint(s.AsciiPort)) }
func (s Settings) HTTPAddr() string  { return net.JoinHostPort(s.Drive, fmt.Sprint(s.HTTPPort)) }

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "xenax-bridge", "settings.json")
}

func loadSettings() Settings {
	s := defaultSettings()
	p := settingsPath()
	if p == "" {
		return s
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return s
	}
	var loaded Settings
	if json.Unmarshal(b, &loaded) == nil && loaded.Validate() == nil {
		return loaded
	}
	return s
}

func saveSettings(s Settings) error {
	p := settingsPath()
	if p == "" {
		return errors.New("no user config directory")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0o644)
}
