// Package config handles mog configuration and token storage.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds mog configuration.
// Compatible with both Go and Node mog formats.
type Config struct {
	ClientID   string `json:"client_id"`  // Go format
	ClientIDv2 string `json:"clientId"`   // Node format
	Storage    string `json:"storage"`    // Token storage: file or keychain
}

// GetClientID returns the client ID, handling both formats.
func (c *Config) GetClientID() string {
	if c.ClientID != "" {
		return c.ClientID
	}
	return c.ClientIDv2
}

// Tokens holds OAuth tokens.
// Compatible with both Go and Node mog formats.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`    // Go format
	ExpiresIn    int64  `json:"expires_in"`    // Node format
	SavedAt      int64  `json:"saved_at"`      // Node format (ms)
}

// GetExpiresAt returns the expiration time, handling both formats.
func (t *Tokens) GetExpiresAt() int64 {
	if t.ExpiresAt > 0 {
		return t.ExpiresAt
	}
	if t.SavedAt > 0 && t.ExpiresIn > 0 {
		// Node format: saved_at is ms, expires_in is seconds
		return (t.SavedAt / 1000) + t.ExpiresIn
	}
	return 0
}

// Slugs holds ID to slug mappings.
type Slugs struct {
	IDToSlug map[string]string `json:"id_to_slug"`
	SlugToID map[string]string `json:"slug_to_id"`
}

// PersonalTenantID is the tenant ID for personal Microsoft accounts (MSA).
// All personal accounts (hotmail.com, live.com, outlook.com) use this tenant.
const PersonalTenantID = "9188040d-6c67-4c5b-b112-36a304b66dad"

// AccountsConfig holds multi-account configuration.
type AccountsConfig struct {
	Default  string                   `json:"default"`  // Default account email
	Accounts map[string]*AccountEntry `json:"accounts"` // email -> account entry
}

// AccountEntry holds metadata for a single account.
type AccountEntry struct {
	Email       string `json:"email"`
	TenantID    string `json:"tenant_id"`    // From JWT "tid" claim
	AccountType string `json:"account_type"` // "personal" or "work"
	AddedAt     int64  `json:"added_at"`     // Unix timestamp
}

// IsPersonal returns true if this is a personal Microsoft account.
func (a *AccountEntry) IsPersonal() bool {
	return a.TenantID == PersonalTenantID || a.AccountType == "personal"
}

// SanitizeEmailForPath converts an email to a filesystem-safe directory name.
// Handles special characters that are invalid in file paths.
func SanitizeEmailForPath(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	// Replace unsafe chars: / \ : * ? " < > |
	replacer := strings.NewReplacer(
		"/", "%2F",
		"\\", "%5C",
		":", "%3A",
		"*", "%2A",
		"?", "%3F",
		"\"", "%22",
		"<", "%3C",
		">", "%3E",
		"|", "%7C",
	)
	return replacer.Replace(email)
}

// AccountDir returns the directory path for a specific account.
// Creates the directory if it doesn't exist.
func AccountDir(email string) (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	accountDir := filepath.Join(dir, "accounts", SanitizeEmailForPath(email))
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		return "", err
	}

	return accountDir, nil
}

// LoadAccounts loads the multi-account configuration.
func LoadAccounts() (*AccountsConfig, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "accounts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &AccountsConfig{
				Accounts: make(map[string]*AccountEntry),
			}, nil
		}
		return nil, err
	}

	var cfg AccountsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Initialize map if nil
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]*AccountEntry)
	}

	return &cfg, nil
}

// SaveAccounts saves the multi-account configuration.
func SaveAccounts(cfg *AccountsConfig) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Initialize map if nil
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]*AccountEntry)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "accounts.json"), data, 0600)
}

// ConfigDir returns the config directory path.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mog"), nil
}

// Load loads the configuration file.
func Load() (*Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save saves the configuration file.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600)
}

// LoadTokens loads OAuth tokens.
func LoadTokens() (*Tokens, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "tokens.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in. Run: mog auth login")
		}
		return nil, err
	}

	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}

// SaveTokens saves OAuth tokens.
func SaveTokens(tokens *Tokens) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "tokens.json"), data, 0600)
}

// DeleteTokens removes stored tokens.
func DeleteTokens() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "tokens.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadSlugs loads the slug mappings.
func LoadSlugs() (*Slugs, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "slugs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Slugs{
				IDToSlug: make(map[string]string),
				SlugToID: make(map[string]string),
			}, nil
		}
		return nil, err
	}

	var slugs Slugs
	if err := json.Unmarshal(data, &slugs); err != nil {
		return nil, err
	}

	if slugs.IDToSlug == nil {
		slugs.IDToSlug = make(map[string]string)
	}
	if slugs.SlugToID == nil {
		slugs.SlugToID = make(map[string]string)
	}

	return &slugs, nil
}

// SaveSlugs saves the slug mappings.
func SaveSlugs(slugs *Slugs) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(slugs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "slugs.json"), data, 0600)
}
