// Package cli defines the command-line interface for mog.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/visionik/mogcli/internal/config"
	"github.com/visionik/mogcli/internal/graph"
)

// ClientFactory is a function that creates a Graph client.
// This allows dependency injection for testing.
type ClientFactory func() (graph.Client, error)

// Root is the top-level CLI structure.
type Root struct {
	// Global flags
	Account string      `help:"Account email to use" env:"MOG_ACCOUNT" short:"a"`
	AIHelp  bool        `name:"ai-help" help:"Show detailed help for AI/LLM agents"`
	JSON    bool        `help:"Output JSON to stdout (best for scripting)" xor:"format"`
	Plain   bool        `help:"Output stable, parseable text to stdout (TSV; no colors)" xor:"format"`
	Verbose bool        `help:"Show full IDs and extra details" short:"v"`
	Force   bool        `help:"Skip confirmations for destructive commands"`
	NoInput bool        `help:"Never prompt; fail instead (useful for CI)" name:"no-input"`
	Version VersionFlag `name:"version" help:"Print version and exit"`

	// Subcommands
	Auth     AuthCmd     `cmd:"" help:"Authentication"`
	Mail     MailCmd     `cmd:"" aliases:"email" help:"Mail operations"`
	Calendar CalendarCmd `cmd:"" aliases:"cal" help:"Calendar operations"`
	Drive    DriveCmd    `cmd:"" help:"OneDrive file operations"`
	Contacts ContactsCmd `cmd:"" help:"Contact operations"`
	Tasks    TasksCmd    `cmd:"" aliases:"todo" help:"Microsoft To-Do tasks"`
	Excel    ExcelCmd    `cmd:"" help:"Excel spreadsheet operations"`
	OneNote  OneNoteCmd  `cmd:"" aliases:"onenote" help:"OneNote operations"`
	Word     WordCmd     `cmd:"" help:"Word document operations"`
	PPT      PPTCmd      `cmd:"" aliases:"ppt,powerpoint" help:"PowerPoint operations"`

	// ClientFactory allows injecting a custom client factory for testing.
	// If nil, graph.NewClient is used.
	ClientFactory ClientFactory `kong:"-"`
}

// ResolveAccount returns the account email to use based on the resolution order:
// 1. --account flag (highest priority)
// 2. MOG_ACCOUNT environment variable
// 3. Default account from config
// 4. Single account auto-select
func (r *Root) ResolveAccount() (string, error) {
	// 1. --account flag (highest priority)
	if r.Account != "" {
		return r.validateAccount(r.Account)
	}

	// 2. MOG_ACCOUNT env var (already handled by Kong via env tag, but check anyway)
	if env := os.Getenv("MOG_ACCOUNT"); env != "" {
		return r.validateAccount(env)
	}

	// 3. Default account from config
	accounts, err := config.LoadAccounts()
	if err != nil {
		return "", fmt.Errorf("failed to load accounts: %w", err)
	}

	if accounts.Default != "" {
		return accounts.Default, nil
	}

	// 4. Single account auto-select
	if len(accounts.Accounts) == 1 {
		for email := range accounts.Accounts {
			return email, nil
		}
	}

	// No account available
	if len(accounts.Accounts) == 0 {
		return "", fmt.Errorf("not logged in. Run: mog auth login <email> --client-id <id>")
	}

	// Multiple accounts but no default
	return "", fmt.Errorf("multiple accounts configured. Specify with --account or set default with: mog auth default <email>")
}

// validateAccount checks that the account exists in the config.
// The email is normalized (lowercased, trimmed) before lookup.
func (r *Root) validateAccount(email string) (string, error) {
	// Normalize email for case-insensitive matching
	email = strings.ToLower(strings.TrimSpace(email))

	accounts, err := config.LoadAccounts()
	if err != nil {
		return "", fmt.Errorf("failed to load accounts: %w", err)
	}

	if _, ok := accounts.Accounts[email]; !ok {
		return "", fmt.Errorf("account %q not found. Run: mog auth list", email)
	}

	return email, nil
}

// GetClient returns a Graph client using the configured factory or default.
func (r *Root) GetClient() (graph.Client, error) {
	if r.ClientFactory != nil {
		return r.ClientFactory()
	}

	email, err := r.ResolveAccount()
	if err != nil {
		// Fall back to legacy single-account behavior if no multi-account config
		accounts, _ := config.LoadAccounts()
		if accounts == nil || len(accounts.Accounts) == 0 {
			return graph.NewClient()
		}
		return nil, err
	}

	return graph.NewClientForAccount(email)
}

// VersionFlag handles --version.
type VersionFlag string

// BeforeApply prints version and exits.
func (v VersionFlag) BeforeApply() error {
	fmt.Println(v)
	os.Exit(0)
	return nil
}
