// Package graph provides slug ID management.
package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/visionik/mogcli/internal/config"
)

var (
	slugCaches     = make(map[string]*config.Slugs) // email -> slug cache
	currentAccount string                           // current account for slug operations
	slugMu         sync.Mutex
)

// SetCurrentAccount sets the current account for slug operations.
func SetCurrentAccount(email string) {
	slugMu.Lock()
	defer slugMu.Unlock()
	currentAccount = email
}

// GetCurrentAccount returns the current account for slug operations.
func GetCurrentAccount() string {
	slugMu.Lock()
	defer slugMu.Unlock()
	return currentAccount
}

// getSlugCache returns the slug cache for the current account, loading if needed.
func getSlugCache() *config.Slugs {
	account := currentAccount
	if account == "" {
		// Fall back to default account or legacy behavior
		accounts, _ := config.LoadAccounts()
		if accounts != nil && accounts.Default != "" {
			account = accounts.Default
		}
	}

	if account != "" {
		if cache, ok := slugCaches[account]; ok {
			return cache
		}

		// Load from account-specific location
		cache, err := config.LoadSlugsForAccount(account)
		if err != nil {
			cache = &config.Slugs{
				IDToSlug: make(map[string]string),
				SlugToID: make(map[string]string),
			}
		}
		slugCaches[account] = cache
		return cache
	}

	// Legacy fallback: use global slug cache
	if legacyCache, ok := slugCaches[""]; ok {
		return legacyCache
	}
	legacyCache, err := config.LoadSlugs()
	if err != nil {
		legacyCache = &config.Slugs{
			IDToSlug: make(map[string]string),
			SlugToID: make(map[string]string),
		}
	}
	slugCaches[""] = legacyCache
	return legacyCache
}

// saveSlugCache saves the slug cache for the current account.
func saveSlugCache(cache *config.Slugs) error {
	account := currentAccount
	if account == "" {
		accounts, _ := config.LoadAccounts()
		if accounts != nil && accounts.Default != "" {
			account = accounts.Default
		}
	}

	if account != "" {
		return config.SaveSlugsForAccount(account, cache)
	}
	// Legacy fallback
	return config.SaveSlugs(cache)
}

// FormatID converts a long Microsoft Graph ID to a short slug.
func FormatID(id string) string {
	if id == "" {
		return ""
	}

	slugMu.Lock()
	defer slugMu.Unlock()

	cache := getSlugCache()

	// Check if we already have a slug for this ID
	if slug, ok := cache.IDToSlug[id]; ok {
		return slug
	}

	// Generate a new slug
	hash := sha256.Sum256([]byte(id))
	slug := hex.EncodeToString(hash[:])[:8]

	// Handle collisions
	origSlug := slug
	counter := 0
	for {
		if existingID, ok := cache.SlugToID[slug]; !ok || existingID == id {
			break
		}
		counter++
		slug = origSlug[:6] + hex.EncodeToString([]byte{byte(counter)})[:2]
	}

	// Store the mapping
	cache.IDToSlug[id] = slug
	cache.SlugToID[slug] = id

	// Save to disk (ignore errors for performance)
	_ = saveSlugCache(cache)

	return slug
}

// ResolveID converts a slug or full ID back to a full ID.
func ResolveID(input string) string {
	if input == "" {
		return ""
	}

	// If it looks like a full ID (long), return as-is
	if len(input) > 16 {
		return input
	}

	slugMu.Lock()
	defer slugMu.Unlock()

	cache := getSlugCache()

	// Try to resolve as a slug
	if fullID, ok := cache.SlugToID[input]; ok {
		return fullID
	}

	// Return as-is (might be a short ID that we haven't seen)
	return input
}

// ClearSlugs clears the slug cache for the current account.
func ClearSlugs() error {
	slugMu.Lock()
	defer slugMu.Unlock()

	cache := &config.Slugs{
		IDToSlug: make(map[string]string),
		SlugToID: make(map[string]string),
	}

	account := currentAccount
	if account == "" {
		accounts, _ := config.LoadAccounts()
		if accounts != nil && accounts.Default != "" {
			account = accounts.Default
		}
	}

	if account != "" {
		slugCaches[account] = cache
		return config.SaveSlugsForAccount(account, cache)
	}

	// Legacy fallback
	slugCaches[""] = cache
	return config.SaveSlugs(cache)
}

// ClearSlugsForAccount clears the slug cache for a specific account.
func ClearSlugsForAccount(email string) error {
	slugMu.Lock()
	defer slugMu.Unlock()

	cache := &config.Slugs{
		IDToSlug: make(map[string]string),
		SlugToID: make(map[string]string),
	}

	slugCaches[email] = cache
	return config.SaveSlugsForAccount(email, cache)
}

// ResetSlugCaches resets all in-memory slug caches (for testing).
func ResetSlugCaches() {
	slugMu.Lock()
	defer slugMu.Unlock()
	slugCaches = make(map[string]*config.Slugs)
	currentAccount = ""
}
