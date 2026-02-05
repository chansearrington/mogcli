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
// Automatically attempts migration from single-account format if needed.
func LoadAccounts() (*AccountsConfig, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "accounts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to migrate from single-account format
			TryMigrate()

			// Try loading again after migration
			data, err = os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					// Return empty config if file still doesn't exist
					return &AccountsConfig{
						Accounts: make(map[string]*AccountEntry),
					}, nil
				}
				return nil, err
			}
		} else {
			return nil, err
		}
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

// atomicWriteJSON writes data to a file atomically using temp file + rename.
// This prevents corruption if the write is interrupted.
func atomicWriteJSON(path string, data interface{}, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // Clean up on error

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	return os.Rename(tmpPath, path)
}

// LoadTokensForAccount loads OAuth tokens for a specific account.
func LoadTokensForAccount(email string) (*Tokens, error) {
	dir, err := AccountDir(email)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "tokens.json")

	// Check file permissions for security (reject world or group readable)
	if info, err := os.Stat(path); err == nil {
		if info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("insecure permissions on %s (got %o, should not be group/world readable)", path, info.Mode().Perm())
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in for %s. Run: mog auth login %s", email, email)
		}
		return nil, err
	}

	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse tokens for %s: %w", email, err)
	}

	return &tokens, nil
}

// SaveTokensForAccount saves OAuth tokens for a specific account using atomic write.
func SaveTokensForAccount(email string, tokens *Tokens) error {
	dir, err := AccountDir(email)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "tokens.json")
	return atomicWriteJSON(path, tokens, 0600)
}

// DeleteTokensForAccount removes stored tokens for a specific account.
func DeleteTokensForAccount(email string) error {
	dir, err := AccountDir(email)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "tokens.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadSlugsForAccount loads the slug mappings for a specific account.
func LoadSlugsForAccount(email string) (*Slugs, error) {
	dir, err := AccountDir(email)
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
		return nil, fmt.Errorf("failed to parse slugs for %s: %w", email, err)
	}

	if slugs.IDToSlug == nil {
		slugs.IDToSlug = make(map[string]string)
	}
	if slugs.SlugToID == nil {
		slugs.SlugToID = make(map[string]string)
	}

	return &slugs, nil
}

// SaveSlugsForAccount saves the slug mappings for a specific account.
func SaveSlugsForAccount(email string, slugs *Slugs) error {
	dir, err := AccountDir(email)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "slugs.json")
	return atomicWriteJSON(path, slugs, 0600)
}

// DeleteSlugsForAccount removes stored slugs for a specific account.
func DeleteSlugsForAccount(email string) error {
	dir, err := AccountDir(email)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "slugs.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// MigrateToMultiAccount migrates single-account config to multi-account format.
// This is called automatically when old tokens exist but no accounts.json.
// It's idempotent - safe to call multiple times.
func MigrateToMultiAccount() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	// Check if already migrated (accounts.json exists with accounts)
	accountsPath := filepath.Join(dir, "accounts.json")
	if data, err := os.ReadFile(accountsPath); err == nil {
		var cfg AccountsConfig
		if json.Unmarshal(data, &cfg) == nil && len(cfg.Accounts) > 0 {
			return nil // Already migrated
		}
	}

	// Check if old tokens exist
	oldTokensPath := filepath.Join(dir, "tokens.json")
	oldTokensData, err := os.ReadFile(oldTokensPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to migrate
		}
		return err
	}

	var tokens Tokens
	if err := json.Unmarshal(oldTokensData, &tokens); err != nil {
		return fmt.Errorf("failed to parse old tokens: %w", err)
	}

	// Use "migrated" as placeholder email if we can't determine it
	email := "migrated"

	// Create backup of old config
	backupDir := filepath.Join(dir, ".backup-migration")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Backup tokens.json
	backupTokensPath := filepath.Join(backupDir, "tokens.json")
	if err := os.WriteFile(backupTokensPath, oldTokensData, 0600); err != nil {
		return fmt.Errorf("failed to backup tokens: %w", err)
	}

	// Backup slugs.json if it exists
	oldSlugsPath := filepath.Join(dir, "slugs.json")
	if slugsData, err := os.ReadFile(oldSlugsPath); err == nil {
		backupSlugsPath := filepath.Join(backupDir, "slugs.json")
		if err := os.WriteFile(backupSlugsPath, slugsData, 0600); err != nil {
			return fmt.Errorf("failed to backup slugs: %w", err)
		}
	}

	// Create account directory and move tokens
	accountDir, err := AccountDir(email)
	if err != nil {
		return fmt.Errorf("failed to create account dir: %w", err)
	}

	// Move tokens to account directory
	newTokensPath := filepath.Join(accountDir, "tokens.json")
	if err := os.WriteFile(newTokensPath, oldTokensData, 0600); err != nil {
		return fmt.Errorf("failed to write account tokens: %w", err)
	}

	// Move slugs if they exist
	if slugsData, err := os.ReadFile(oldSlugsPath); err == nil {
		newSlugsPath := filepath.Join(accountDir, "slugs.json")
		if err := os.WriteFile(newSlugsPath, slugsData, 0600); err != nil {
			return fmt.Errorf("failed to write account slugs: %w", err)
		}
		// Remove old slugs file
		_ = os.Remove(oldSlugsPath)
	}

	// Create accounts.json with migrated account
	accounts := &AccountsConfig{
		Default: email,
		Accounts: map[string]*AccountEntry{
			email: {
				Email:       email,
				AccountType: "work", // Assume work account
				AddedAt:     0,      // Unknown
			},
		},
	}

	if err := SaveAccounts(accounts); err != nil {
		return fmt.Errorf("failed to save accounts: %w", err)
	}

	// Remove old tokens file (backup exists)
	_ = os.Remove(oldTokensPath)

	return nil
}

// TryMigrate attempts to migrate from single-account to multi-account format.
// This is a no-op if already migrated or no old tokens exist.
// Errors are logged but not returned to avoid blocking normal operation.
func TryMigrate() {
	if err := MigrateToMultiAccount(); err != nil {
		// Log but don't fail - user can still use the CLI
		fmt.Fprintf(os.Stderr, "Warning: migration failed: %v\n", err)
	}
}
