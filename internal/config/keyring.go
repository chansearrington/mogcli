package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "mog"
	tokenKey    = "oauth_tokens"
)

// StorageType represents the credential storage backend.
type StorageType string

const (
	StorageFile    StorageType = "file"
	StorageKeyring StorageType = "keyring"
)

// DefaultStorage is the default storage type.
var DefaultStorage = StorageFile

// CurrentStorage holds the active storage type.
var CurrentStorage = DefaultStorage

// SetStorage sets the storage type for credentials.
func SetStorage(st StorageType) {
	CurrentStorage = st
}

// SaveTokensKeyring saves OAuth tokens to the system keyring.
func SaveTokensKeyring(tokens *Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}
	return keyring.Set(serviceName, tokenKey, string(data))
}

// LoadTokensKeyring loads OAuth tokens from the system keyring.
func LoadTokensKeyring() (*Tokens, error) {
	data, err := keyring.Get(serviceName, tokenKey)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, fmt.Errorf("not logged in. Run: mog auth login")
		}
		return nil, fmt.Errorf("failed to get tokens from keyring: %w", err)
	}

	var tokens Tokens
	if err := json.Unmarshal([]byte(data), &tokens); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
	}
	return &tokens, nil
}

// DeleteTokensKeyring removes OAuth tokens from the system keyring.
func DeleteTokensKeyring() error {
	err := keyring.Delete(serviceName, tokenKey)
	if err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("failed to delete tokens from keyring: %w", err)
	}
	return nil
}

// SaveTokensAuto saves tokens using the current storage type.
func SaveTokensAuto(tokens *Tokens) error {
	switch CurrentStorage {
	case StorageKeyring:
		return SaveTokensKeyring(tokens)
	default:
		return SaveTokens(tokens)
	}
}

// LoadTokensAuto loads tokens using the current storage type.
func LoadTokensAuto() (*Tokens, error) {
	switch CurrentStorage {
	case StorageKeyring:
		return LoadTokensKeyring()
	default:
		return LoadTokens()
	}
}

// DeleteTokensAuto deletes tokens using the current storage type.
func DeleteTokensAuto() error {
	switch CurrentStorage {
	case StorageKeyring:
		return DeleteTokensKeyring()
	default:
		return DeleteTokens()
	}
}

// keyringKeyForAccount generates a namespaced keyring key for an account.
// Format: mog:<client_hash>:<email>
// This allows future support for multiple client IDs per account.
func keyringKeyForAccount(email string) string {
	cfg, _ := Load()
	clientID := cfg.GetClientID()
	if clientID == "" {
		clientID = "default"
	}
	clientHash := fmt.Sprintf("%x", sha256.Sum256([]byte(clientID)))[:8]
	return fmt.Sprintf("%s:%s:%s", serviceName, clientHash, email)
}

// SaveTokensKeyringForAccount saves OAuth tokens to the system keyring for a specific account.
func SaveTokensKeyringForAccount(email string, tokens *Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}
	key := keyringKeyForAccount(email)
	return keyring.Set(serviceName, key, string(data))
}

// LoadTokensKeyringForAccount loads OAuth tokens from the system keyring for a specific account.
func LoadTokensKeyringForAccount(email string) (*Tokens, error) {
	key := keyringKeyForAccount(email)
	data, err := keyring.Get(serviceName, key)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, fmt.Errorf("not logged in for %s. Run: mog auth login %s", email, email)
		}
		return nil, fmt.Errorf("failed to get tokens from keyring for %s: %w", email, err)
	}

	var tokens Tokens
	if err := json.Unmarshal([]byte(data), &tokens); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tokens for %s: %w", email, err)
	}
	return &tokens, nil
}

// DeleteTokensKeyringForAccount removes OAuth tokens from the system keyring for a specific account.
func DeleteTokensKeyringForAccount(email string) error {
	key := keyringKeyForAccount(email)
	err := keyring.Delete(serviceName, key)
	if err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("failed to delete tokens from keyring for %s: %w", email, err)
	}
	return nil
}

// SaveTokensAutoForAccount saves tokens for an account using the current storage type.
func SaveTokensAutoForAccount(email string, tokens *Tokens) error {
	switch CurrentStorage {
	case StorageKeyring:
		return SaveTokensKeyringForAccount(email, tokens)
	default:
		return SaveTokensForAccount(email, tokens)
	}
}

// LoadTokensAutoForAccount loads tokens for an account using the current storage type.
func LoadTokensAutoForAccount(email string) (*Tokens, error) {
	switch CurrentStorage {
	case StorageKeyring:
		return LoadTokensKeyringForAccount(email)
	default:
		return LoadTokensForAccount(email)
	}
}

// DeleteTokensAutoForAccount deletes tokens for an account using the current storage type.
func DeleteTokensAutoForAccount(email string) error {
	switch CurrentStorage {
	case StorageKeyring:
		return DeleteTokensKeyringForAccount(email)
	default:
		return DeleteTokensForAccount(email)
	}
}
