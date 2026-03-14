package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const appDir = "gimble"
const configFile = "config.json"

type Profile struct {
	Name                   string   `json:"name"`
	Email                  string   `json:"email"`
	GitHub                 string   `json:"github"`
	Provider               string   `json:"provider,omitempty"`
	WorkspaceRoots         []string `json:"workspace_roots,omitempty"`
	ROSType                string   `json:"ros_type,omitempty"`
	ROSDistro              string   `json:"ros_distro,omitempty"`
	ROSWorkspace           string   `json:"ros_workspace,omitempty"`
	ObsGrafanaURL          string   `json:"obs_grafana_url,omitempty"`
	ObsSentryURL           string   `json:"obs_sentry_url,omitempty"`
	SystemPromptProfile    string   `json:"system_prompt_profile,omitempty"`
	NotificationPreference string   `json:"notification_preference,omitempty"`
}

type Config struct {
	ActiveProfile string             `json:"active_profile"`
	Profiles      map[string]Profile `json:"profiles"`
}

func ConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDir, configFile), nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{Profiles: map[string]Profile{}}, nil
		}
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}

	return cfg, nil
}

func Save(cfg Config) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func NormalizeProfileName(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

func NormalizeGitHub(v string) string {
	v = strings.TrimSpace(v)
	return strings.TrimPrefix(v, "@")
}

func NormalizeProvider(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "gitlab" {
		return "gitlab"
	}
	return "github"
}

func ValidateEmail(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("invalid email: %q", v)
	}
	if _, err := mail.ParseAddress(v); err != nil {
		return fmt.Errorf("invalid email: %q", v)
	}
	parts := strings.Split(v, "@")
	if len(parts) != 2 || !strings.Contains(parts[1], ".") {
		return fmt.Errorf("invalid email: %q", v)
	}
	return nil
}

func (c *Config) Upsert(profileName string, p Profile) {
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	c.Profiles[profileName] = p
}

func (c *Config) Use(profileName string) error {
	if _, ok := c.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}
	c.ActiveProfile = profileName
	return nil
}

func (c *Config) Delete(profileName string) error {
	if _, ok := c.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}
	delete(c.Profiles, profileName)
	if c.ActiveProfile == profileName {
		c.ActiveProfile = ""
	}
	return nil
}

func (c Config) Get(profileName string) (Profile, bool) {
	p, ok := c.Profiles[profileName]
	return p, ok
}

func (c Config) Active() (string, Profile, bool) {
	if c.ActiveProfile == "" {
		return "", Profile{}, false
	}
	p, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		return "", Profile{}, false
	}
	return c.ActiveProfile, p, true
}

func (c Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
