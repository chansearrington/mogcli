package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/visionik/mogcli/internal/config"
	"github.com/visionik/mogcli/internal/graph"
)

// AuthCmd handles authentication commands.
type AuthCmd struct {
	Login   AuthLoginCmd   `cmd:"" help:"Login to Microsoft 365"`
	Status  AuthStatusCmd  `cmd:"" help:"Show authentication status"`
	Logout  AuthLogoutCmd  `cmd:"" help:"Logout and clear tokens"`
	List    AuthListCmd    `cmd:"" help:"List all configured accounts"`
	Default AuthDefaultCmd `cmd:"" help:"Set the default account"`
}

// AuthLoginCmd logs in to Microsoft 365.
type AuthLoginCmd struct {
	Email    string `arg:"" help:"Account email (e.g., user@example.com)" required:""`
	ClientID string `help:"Azure AD client ID" env:"MOG_CLIENT_ID" name:"client-id"`
	Storage  string `help:"Token storage: file or keychain" default:"file" enum:"file,keychain"`
}

// Run executes the auth login command.
func (c *AuthLoginCmd) Run(root *Root) error {
	// Normalize email
	email := strings.ToLower(strings.TrimSpace(c.Email))

	// Load existing config to get client ID if not provided
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}

	clientID := c.ClientID
	if clientID == "" {
		clientID = cfg.GetClientID()
	}
	if clientID == "" {
		return fmt.Errorf("--client-id is required for first login. Use: mog auth login %s --client-id <id>", email)
	}

	// Set storage type
	if c.Storage == "keychain" {
		config.SetStorage(config.StorageKeyring)
	} else {
		config.SetStorage(config.StorageFile)
	}

	// Save client ID and storage preference
	cfg.ClientID = clientID
	cfg.Storage = c.Storage
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Request device code
	fmt.Printf("Requesting device code for %s...\n", email)
	dcResp, err := graph.RequestDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}

	fmt.Println()
	fmt.Println(dcResp.Message)
	fmt.Println()

	// Try to open browser
	openBrowser(dcResp.VerificationURI)

	// Poll for token
	fmt.Println("Waiting for authorization...")
	tokens, err := graph.PollForToken(clientID, dcResp.DeviceCode, dcResp.Interval)
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}

	// Save tokens to account-specific location
	if err := config.SaveTokensAutoForAccount(email, tokens); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	// Load and update accounts config
	accounts, err := config.LoadAccounts()
	if err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}

	// Add or update account entry
	accounts.Accounts[email] = &config.AccountEntry{
		Email:       email,
		AccountType: "work", // Default; could be detected from token
		AddedAt:     time.Now().Unix(),
	}

	// Set as default if first account
	if accounts.Default == "" {
		accounts.Default = email
		fmt.Printf("Set %s as default account\n", email)
	}

	if err := config.SaveAccounts(accounts); err != nil {
		return fmt.Errorf("failed to save accounts: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Successfully logged in as %s (storage: %s)\n", email, c.Storage)
	return nil
}

// AuthStatusCmd shows authentication status.
type AuthStatusCmd struct {
	Email string `arg:"" optional:"" help:"Account email (defaults to current account)"`
}

// Run executes the auth status command.
func (c *AuthStatusCmd) Run(root *Root) error {
	// Load config to get storage preference
	cfg, _ := config.Load()
	if cfg != nil && cfg.Storage == "keychain" {
		config.SetStorage(config.StorageKeyring)
	}

	// Determine which account to show status for
	email := c.Email
	if email == "" {
		// Try to resolve account
		resolved, err := root.ResolveAccount()
		if err != nil {
			// Fall back to legacy single-account behavior
			tokens, err := config.LoadTokensAuto()
			if err != nil {
				fmt.Println("Status: Not logged in")
				return nil
			}
			showLegacyStatus(tokens, cfg)
			return nil
		}
		email = resolved
	}

	// Show status for specific account
	tokens, err := config.LoadTokensAutoForAccount(email)
	if err != nil {
		fmt.Printf("Status: Not logged in (%s)\n", email)
		return nil
	}

	accounts, _ := config.LoadAccounts()
	isDefault := accounts != nil && accounts.Default == email

	fmt.Printf("Account: %s", email)
	if isDefault {
		fmt.Print(" (default)")
	}
	fmt.Println()

	fmt.Println("Status: Logged in")
	if cfg != nil && cfg.Storage != "" {
		fmt.Printf("Storage: %s\n", cfg.Storage)
	}

	if tokens.ExpiresAt > 0 {
		expiresAt := time.Unix(tokens.ExpiresAt, 0)
		remaining := time.Until(expiresAt)
		if remaining > 0 {
			fmt.Printf("Token expires: %s (in %v)\n", expiresAt.Format(time.RFC3339), remaining.Round(time.Minute))
		} else {
			fmt.Println("Token: Expired (will refresh on next request)")
		}
	}

	if cfg != nil && cfg.ClientID != "" {
		fmt.Printf("Client ID: %s...%s\n", cfg.ClientID[:8], cfg.ClientID[len(cfg.ClientID)-4:])
	}

	return nil
}

// showLegacyStatus shows status for legacy single-account config.
func showLegacyStatus(tokens *config.Tokens, cfg *config.Config) {
	fmt.Println("Status: Logged in (legacy single-account)")
	if cfg != nil && cfg.Storage != "" {
		fmt.Printf("Storage: %s\n", cfg.Storage)
	}

	if tokens.ExpiresAt > 0 {
		expiresAt := time.Unix(tokens.ExpiresAt, 0)
		remaining := time.Until(expiresAt)
		if remaining > 0 {
			fmt.Printf("Token expires: %s (in %v)\n", expiresAt.Format(time.RFC3339), remaining.Round(time.Minute))
		} else {
			fmt.Println("Token: Expired (will refresh on next request)")
		}
	}

	if cfg != nil && cfg.ClientID != "" {
		fmt.Printf("Client ID: %s...%s\n", cfg.ClientID[:8], cfg.ClientID[len(cfg.ClientID)-4:])
	}
}

// AuthLogoutCmd logs out and clears tokens.
type AuthLogoutCmd struct {
	Email string `arg:"" optional:"" help:"Account email (defaults to current account)"`
	All   bool   `help:"Logout from all accounts" name:"all"`
}

// Run executes the auth logout command.
func (c *AuthLogoutCmd) Run(root *Root) error {
	// Load config to get storage preference
	cfg, _ := config.Load()
	if cfg != nil && cfg.Storage == "keychain" {
		config.SetStorage(config.StorageKeyring)
	}

	accounts, err := config.LoadAccounts()
	if err != nil {
		accounts = &config.AccountsConfig{Accounts: make(map[string]*config.AccountEntry)}
	}

	// Handle --all flag
	if c.All {
		for email := range accounts.Accounts {
			if err := logoutAccount(email); err != nil {
				fmt.Printf("Warning: failed to logout %s: %v\n", email, err)
			} else {
				fmt.Printf("✓ Logged out %s\n", email)
			}
		}
		// Clear accounts config
		accounts.Accounts = make(map[string]*config.AccountEntry)
		accounts.Default = ""
		if err := config.SaveAccounts(accounts); err != nil {
			return fmt.Errorf("failed to save accounts: %w", err)
		}

		// Also clear legacy tokens
		_ = config.DeleteTokensAuto()
		_ = graph.ClearSlugs()

		fmt.Println("✓ Logged out from all accounts")
		return nil
	}

	// Determine which account to logout
	email := c.Email
	if email == "" {
		// Try to resolve account
		resolved, err := root.ResolveAccount()
		if err != nil {
			// Fall back to legacy single-account behavior
			if err := config.DeleteTokensAuto(); err != nil {
				return fmt.Errorf("failed to delete tokens: %w", err)
			}
			if err := graph.ClearSlugs(); err != nil {
				return fmt.Errorf("failed to clear slugs: %w", err)
			}
			fmt.Println("✓ Logged out successfully")
			return nil
		}
		email = resolved
	}

	// Logout specific account
	if err := logoutAccount(email); err != nil {
		return err
	}

	// Remove from accounts config
	delete(accounts.Accounts, email)

	// Update default if needed
	if accounts.Default == email {
		accounts.Default = ""
		// Auto-select new default if only one account remains
		if len(accounts.Accounts) == 1 {
			for newDefault := range accounts.Accounts {
				accounts.Default = newDefault
				fmt.Printf("New default account: %s\n", newDefault)
			}
		} else if len(accounts.Accounts) > 1 {
			fmt.Println("Note: No default account set. Use: mog auth default <email>")
		}
	}

	if err := config.SaveAccounts(accounts); err != nil {
		return fmt.Errorf("failed to save accounts: %w", err)
	}

	fmt.Printf("✓ Logged out %s\n", email)
	return nil
}

// logoutAccount clears tokens and slugs for a specific account.
func logoutAccount(email string) error {
	if err := config.DeleteTokensAutoForAccount(email); err != nil {
		return fmt.Errorf("failed to delete tokens: %w", err)
	}
	if err := graph.ClearSlugsForAccount(email); err != nil {
		return fmt.Errorf("failed to clear slugs: %w", err)
	}
	return nil
}

// AuthListCmd lists all configured accounts.
type AuthListCmd struct{}

// Run executes the auth list command.
func (c *AuthListCmd) Run(root *Root) error {
	cfg, _ := config.Load()
	if cfg != nil && cfg.Storage == "keychain" {
		config.SetStorage(config.StorageKeyring)
	}

	accounts, err := config.LoadAccounts()
	if err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}

	if len(accounts.Accounts) == 0 {
		fmt.Println("No accounts configured. Run: mog auth login <email> --client-id <id>")
		return nil
	}

	for email, entry := range accounts.Accounts {
		marker := "  "
		if email == accounts.Default {
			marker = "* "
		}

		// Check token status
		tokens, err := config.LoadTokensAutoForAccount(email)
		status := "[no token]"
		if err == nil {
			expiresAt := tokens.GetExpiresAt()
			if expiresAt > 0 && time.Now().Unix() >= expiresAt {
				status = "[expired]"
			} else {
				status = "[valid]"
			}
		}

		accountType := ""
		if entry.AccountType != "" {
			accountType = fmt.Sprintf(" (%s)", entry.AccountType)
		}

		fmt.Printf("%s%s%s %s\n", marker, email, accountType, status)
	}

	return nil
}

// AuthDefaultCmd sets the default account.
type AuthDefaultCmd struct {
	Email string `arg:"" help:"Account email to set as default" required:""`
}

// Run executes the auth default command.
func (c *AuthDefaultCmd) Run(root *Root) error {
	accounts, err := config.LoadAccounts()
	if err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}

	email := strings.ToLower(strings.TrimSpace(c.Email))

	if _, ok := accounts.Accounts[email]; !ok {
		return fmt.Errorf("account %q not found. Run: mog auth list", email)
	}

	accounts.Default = email
	if err := config.SaveAccounts(accounts); err != nil {
		return fmt.Errorf("failed to save accounts: %w", err)
	}

	fmt.Printf("✓ Default account set to %s\n", email)
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	_ = cmd.Start()
}
