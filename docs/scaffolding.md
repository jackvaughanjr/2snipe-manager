# Scaffolding templates

> **Applies to:** `*2snipe` integration authors

These files contain vendor-specific placeholders marked with `TODO` or angle-bracket
tokens (`<vendor>`, `<repo>`, `<module>`, `<Vendor>`). Replace them before building.

---

## go.mod

```
module github.com/<owner>/<repo>

go 1.25

require (
    github.com/spf13/cobra v1.10.2
    github.com/spf13/viper v1.21.0
    golang.org/x/time v0.5.0
)
```

Run `go mod tidy` after creating this file to populate `go.sum` and pin indirect
dependencies. Never create `go.sum` manually.

---

## .gitignore

```
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
go.work
go.work.sum
.env
settings.yaml
*.json
.cache/
<repo>
.DS_Store
```

---

## settings.example.yaml

```yaml
# <repo> configuration
# Copy this file to settings.yaml and fill in your values.
# settings.yaml is gitignored and should never be committed.
#
# All values can be overridden with environment variables.
# See CONTEXT.md for the full list of env var overrides.

<vendor>:
  # Base URL of the source system API.
  url: "https://your-instance.example.com"

  # API credential for the source system.
  api_token: ""

snipe_it:
  # Base URL of your Snipe-IT instance.
  url: "https://your-snipe-it-instance.example.com"

  # Snipe-IT API key with license management permissions.
  # Generate one at: Admin → API Keys
  api_key: ""

  # Name of the license to assign seats to.
  # Created automatically on first run if it does not exist.
  license_name: "<Vendor Product Name>"

  # Snipe-IT category ID to assign to the license when created. Required.
  # Find IDs at: Admin → Categories, or via the Snipe-IT API.
  license_category_id: 0

  # Optional: total purchased seat count for the license.
  # Use when the vendor API does not expose purchased seat count. If 0 and the
  # vendor API also returns nothing, active member count is used as the floor.
  # Seats are never shrunk automatically.
  license_seats: 0

  # Optional: manufacturer ID. If 0, auto find/create from vendor name.
  license_manufacturer_id: 0

  # Optional: supplier ID. If 0, no supplier is set on the license.
  license_supplier_id: 0

sync:
  # Simulate a full sync without making any changes.
  # Overridden by the --dry-run flag.
  dry_run: false

  # Re-sync seat notes even if they appear up to date.
  # Overridden by the --force flag.
  force: false

  # Snipe-IT API rate limit in milliseconds between requests.
  # 500ms = 2 req/s. Increase if you encounter 429 errors.
  rate_limit_ms: 500

slack:
  # Incoming webhook URL for Slack notifications.
  # Optional. If omitted, all notifications are silently skipped.
  # Can be overridden with the SLACK_WEBHOOK environment variable.
  webhook_url: ""
```

---

## main.go

```go
package main

import "github.com/<owner>/<repo>/cmd"

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
```

---

## cmd/root.go

```go
package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "<repo>",
	Short: "Sync active <Vendor> users into Snipe-IT as license seat assignments",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Suppress cobra's duplicate "Error: ..." echo for runtime errors.
		// SilenceUsage is NOT set here — cobra validates Args after PersistentPreRunE,
		// so setting it here would also silence the usage block on missing-arg errors.
		// Set SilenceUsage inside each RunE via the silentUsage wrapper instead.
		cmd.Root().SilenceErrors = true
		initLogging()
		return nil
	},
}

// SetVersion injects the build-time version string into the root command.
func SetVersion(v string) {
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// silentUsage wraps a RunE function so that SilenceUsage is set before the
// function body runs. Because cobra validates Args before calling RunE, arg/flag
// errors still print the usage block; only errors returned from RunE itself
// are silenced.
func silentUsage(fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.Root().SilenceUsage = true
		return fn(cmd, args)
	}
}

// fatal prints an error to stderr and returns it. Use in RunE instead of bare
// return fmt.Errorf(...) so errors are visible when SilenceErrors is set.
func fatal(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return err
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "settings.yaml", "path to config file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "INFO-level logging")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "DEBUG-level logging")
	rootCmd.PersistentFlags().String("log-file", "", "append logs to this file")
	rootCmd.PersistentFlags().String("log-format", "text", "log format: text or json")

	_ = viper.BindPFlag("log.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("log.debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("log.file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
}

func initConfig() {
	viper.SetConfigFile(cfgFile)
	viper.SetConfigType("yaml")

	// TODO: add vendor-specific env var bindings here.
	// Standard bindings (keep these for all integrations):
	viper.BindEnv("<vendor>.url", "<VENDOR>_URL")         // TODO: replace tokens
	viper.BindEnv("<vendor>.api_token", "<VENDOR>_TOKEN") // TODO: replace tokens
	viper.BindEnv("snipe_it.url", "SNIPE_URL")
	viper.BindEnv("snipe_it.api_key", "SNIPE_TOKEN")
	viper.BindEnv("slack.webhook_url", "SLACK_WEBHOOK")
	_ = viper.BindEnv("sync.rate_limit_ms", "SNIPE_RATE_LIMIT_MS")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Warn("could not read config file", "error", err)
		}
	}
}

func initLogging() {
	level := slog.LevelWarn
	if viper.GetBool("log.debug") {
		level = slog.LevelDebug
	} else if viper.GetBool("log.verbose") {
		level = slog.LevelInfo
	}

	var w io.Writer = os.Stderr
	if path := viper.GetString("log.file"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Warn("could not open log file", "path", path, "error", err)
		} else {
			w = io.MultiWriter(os.Stderr, f)
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if viper.GetString("log.format") == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	slog.SetDefault(slog.New(handler))
}
```

---

## cmd/sync.go

```go
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	// TODO: replace with your vendor package import path
	vendor "<module>/internal/<vendor>"
	"<module>/internal/slack"
	"<module>/internal/snipeit"
	"<module>/internal/sync"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync active <Vendor> users into Snipe-IT license seats",
	RunE:  silentUsage(runSync),
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().Bool("dry-run", false, "simulate without making changes")
	syncCmd.Flags().Bool("force", false, "re-sync even if notes appear up to date")
	syncCmd.Flags().String("email", "", "sync a single user by email address")
	syncCmd.Flags().Bool("create-users", false, "create Snipe-IT accounts for unmatched users")
	syncCmd.Flags().Bool("no-slack", false, "suppress Slack notifications for this run")

	_ = viper.BindPFlag("sync.dry_run", syncCmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("sync.force", syncCmd.Flags().Lookup("force"))
	_ = viper.BindPFlag("sync.create_users", syncCmd.Flags().Lookup("create-users"))
}

func runSync(cmd *cobra.Command, args []string) error {
	// TODO: replace with your vendor client constructor and config keys
	vendorClient := vendor.NewClient(
		viper.GetString("<vendor>.url"),
		viper.GetString("<vendor>.api_token"),
	)
	rateLimitMs := viper.GetInt("sync.rate_limit_ms")
	if rateLimitMs <= 0 {
		rateLimitMs = 500
	}
	snipeClient := snipeit.NewClient(
		viper.GetString("snipe_it.url"),
		viper.GetString("snipe_it.api_key"),
		rateLimitMs,
	)

	emailFilter, _ := cmd.Flags().GetString("email")
	noSlack, _ := cmd.Flags().GetBool("no-slack")

	categoryID := viper.GetInt("snipe_it.license_category_id")
	if categoryID == 0 {
		return fatal("snipe_it.license_category_id is required in settings.yaml")
	}

	cfg := sync.Config{
		DryRun:            viper.GetBool("sync.dry_run"),
		Force:             viper.GetBool("sync.force"),
		CreateUsers:       viper.GetBool("sync.create_users"),
		LicenseName:       viper.GetString("snipe_it.license_name"),
		LicenseCategoryID: categoryID,
		ManufacturerID:    viper.GetInt("snipe_it.license_manufacturer_id"),
		SupplierID:        viper.GetInt("snipe_it.license_supplier_id"),
	}

	if cfg.LicenseName == "" {
		cfg.LicenseName = "<Vendor Product Name>" // TODO: replace with default license name
	}

	if cfg.DryRun {
		slog.Info("dry-run mode enabled — no changes will be made")
	}

	slackClient := slack.NewClient(viper.GetString("slack.webhook_url"))
	ctx := context.Background()

	syncer := sync.NewSyncer(vendorClient, snipeClient, cfg)
	result, err := syncer.Run(ctx, emailFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
		if !cfg.DryRun && !noSlack {
			msg := fmt.Sprintf("<repo> sync failed: %v", err) // TODO: replace <repo>
			if notifyErr := slackClient.Send(ctx, msg); notifyErr != nil {
				slog.Warn("slack notification failed", "error", notifyErr)
			}
		}
		return err
	}

	if !cfg.DryRun && !noSlack {
		for _, email := range result.UnmatchedEmails {
			// TODO: replace <repo> and <Vendor> in message
			msg := fmt.Sprintf("<repo>: no Snipe-IT account found for <Vendor> user — %s", email)
			if notifyErr := slackClient.Send(ctx, msg); notifyErr != nil {
				slog.Warn("slack notification failed", "email", email, "error", notifyErr)
			}
		}

		// TODO: replace <repo> in message
		msg := fmt.Sprintf(
			"<repo> sync complete — checked out: %d, notes updated: %d, checked in: %d, skipped: %d, users created: %d, warnings: %d",
			result.CheckedOut, result.NotesUpdated, result.CheckedIn, result.Skipped, result.UsersCreated, result.Warnings,
		)
		if notifyErr := slackClient.Send(ctx, msg); notifyErr != nil {
			slog.Warn("slack notification failed", "error", notifyErr)
		}
	}

	fmt.Printf("Sync complete: checked_out=%d notes_updated=%d checked_in=%d skipped=%d users_created=%d warnings=%d\n",
		result.CheckedOut, result.NotesUpdated, result.CheckedIn, result.Skipped, result.UsersCreated, result.Warnings)
	return nil
}
```

---

## cmd/test.go

```go
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	// TODO: replace with your vendor package import path
	vendor "<module>/internal/<vendor>"
	"<module>/internal/snipeit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Validate API connections and report current state",
	RunE:  silentUsage(runTest),
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// TODO: replace with your vendor client constructor and config keys
	vendorClient := vendor.NewClient(
		viper.GetString("<vendor>.url"),
		viper.GetString("<vendor>.api_token"),
	)
	rateLimitMs := viper.GetInt("sync.rate_limit_ms")
	if rateLimitMs <= 0 {
		rateLimitMs = 500
	}
	snipeClient := snipeit.NewClient(
		viper.GetString("snipe_it.url"),
		viper.GetString("snipe_it.api_key"),
		rateLimitMs,
	)

	// --- Vendor ---
	fmt.Println("=== <Vendor> ===") // TODO: replace <Vendor>
	slog.Info("fetching active users", "url", viper.GetString("<vendor>.url"))
	users, err := vendorClient.ListActiveUsers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "<Vendor> error: %v\n", err) // TODO: replace <Vendor>
		return err
	}
	fmt.Printf("Active users: %d\n", len(users))

	// TODO: add vendor-specific output (role counts, group counts, etc.)

	// --- Snipe-IT ---
	fmt.Println("\n=== Snipe-IT ===")
	licenseName := viper.GetString("snipe_it.license_name")
	if licenseName == "" {
		licenseName = "<Vendor Product Name>" // TODO: replace with default
	}

	slog.Info("looking up license in Snipe-IT", "url", viper.GetString("snipe_it.url"), "license", licenseName)
	lic, err := snipeClient.FindLicenseByName(ctx, licenseName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Snipe-IT error: %v\n", err)
		return err
	}
	if lic == nil {
		fmt.Printf("License %q: not found\n", licenseName)
	} else {
		slog.Debug("license detail", "id", lic.ID, "seats", lic.Seats, "free", lic.FreeSeatsCount)
		fmt.Printf("License %q: id=%d seats=%d free=%d\n",
			lic.Name, lic.ID, lic.Seats, lic.FreeSeatsCount)
	}

	return nil
}
```

---

## internal/sync/syncer.go

The syncer is the only file with significant vendor-specific logic. The overall
structure is fixed; vendor-specific sections are marked with `TODO`.

The `Config` struct, `Syncer` struct, and `NewSyncer` constructor are fully generic.
The only vendor-specific parts are:
- The import and type of the vendor client field
- Step 4 (per-user metadata fetch)
- Step 5 (manufacturer resolution — may not apply to all vendors)
- `emailKey(user)` — must match your vendor user struct's email field
- `buildNotes(metadata)` — formats vendor metadata into the seat notes string

```go
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	// TODO: replace with your vendor package import path
	vendor "<module>/internal/<vendor>"
	"<module>/internal/snipeit"
)

// Config controls sync behaviour.
type Config struct {
	DryRun      bool
	Force       bool
	CreateUsers bool
	LicenseName       string
	LicenseCategoryID int
	// LicenseSeats is the total purchased seat count. Resolution priority:
	// vendor API (if available) → this field → active member count (floor).
	// If 0 and vendor API returns nothing, active member count is used.
	LicenseSeats int
	// ManufacturerID is optional. If 0, auto find/create from vendor name.
	ManufacturerID int
	// SupplierID is optional. If 0, no supplier is set on the license.
	SupplierID int
}

// Syncer orchestrates the source → Snipe-IT license sync.
type Syncer struct {
	vendor *vendor.Client // TODO: replace with your vendor client type
	snipe  *snipeit.Client
	config Config
}

func NewSyncer(v *vendor.Client, snipe *snipeit.Client, cfg Config) *Syncer { // TODO: update type
	return &Syncer{vendor: v, snipe: snipe, config: cfg}
}

// Run executes the full sync. emailFilter restricts the checkout pass to one
// user (and skips the checkin pass entirely).
func (s *Syncer) Run(ctx context.Context, emailFilter string) (Result, error) {
	var result Result

	// 1. Fetch active users from the source system (paginated).
	slog.Info("fetching active users")
	activeUsers, err := s.vendor.ListActiveUsers(ctx)
	if err != nil {
		return result, err
	}
	slog.Info("fetched active users", "count", len(activeUsers))

	// 2. Build active email set (used in the checkin pass).
	activeEmails := make(map[string]struct{}, len(activeUsers))
	for _, u := range activeUsers {
		activeEmails[emailKey(u)] = struct{}{}
	}

	// 3. Apply --email filter.
	if emailFilter != "" {
		needle := strings.ToLower(emailFilter)
		filtered := activeUsers[:0]
		for _, u := range activeUsers {
			if emailKey(u) == needle {
				filtered = append(filtered, u)
				break
			}
		}
		activeUsers = filtered
		slog.Info("filtered to single user", "email", emailFilter, "found", len(activeUsers) > 0)
	}

	// 4. TODO: Fetch per-user metadata (roles, groups, licenses, etc.).
	// Example pattern:
	//   type userWithMeta struct {
	//       user  vendor.User
	//       meta  []vendor.Role  // or whatever your vendor provides
	//   }
	//   usersWithMeta := make([]userWithMeta, 0, len(activeUsers))
	//   for _, u := range activeUsers {
	//       meta, err := s.vendor.GetUserRoles(ctx, u.ID)
	//       if err != nil {
	//           slog.Warn("could not fetch metadata", "user", emailKey(u), "error", err)
	//       }
	//       usersWithMeta = append(usersWithMeta, userWithMeta{u, meta})
	//   }

	// 5. TODO: Resolve any Snipe-IT entities needed before license creation.
	// Common pattern — find or create the manufacturer:
	//   manufacturerID := s.config.ManufacturerID
	//   if !s.config.DryRun && manufacturerID == 0 {
	//       mfr, err := s.snipe.FindOrCreateManufacturer(ctx, "<Vendor>", "https://vendor.example.com")
	//       if err != nil { return result, err }
	//       manufacturerID = mfr.ID
	//   }
	manufacturerID := s.config.ManufacturerID

	// 6. Resolve target seat count.
	// Priority: vendor API (if available) → config override → active member count (floor).
	// TODO: replace 0 with a vendor API call if the vendor exposes purchased seat count.
	activeCount := len(activeEmails)
	vendorLicenseSeats := 0 // TODO: fetch from vendor API if available
	targetSeats := vendorLicenseSeats
	if targetSeats == 0 {
		targetSeats = s.config.LicenseSeats
	}
	if targetSeats == 0 {
		targetSeats = activeCount
	} else if targetSeats < activeCount {
		slog.Warn("license seat count is less than active member count; using active count",
			"license_seats", targetSeats, "active", activeCount)
		targetSeats = activeCount
	}

	// Find or create the license.
	// Dry-run: find only; synthesize placeholder if not found (id=0).
	slog.Info("finding or creating license", "name", s.config.LicenseName)
	var lic *snipeit.License
	if s.config.DryRun {
		lic, err = s.snipe.FindLicenseByName(ctx, s.config.LicenseName)
		if err != nil {
			return result, err
		}
		if lic == nil {
			slog.Info("[dry-run] license not found; would be created", "name", s.config.LicenseName, "seats", targetSeats)
			lic = &snipeit.License{Name: s.config.LicenseName, Seats: targetSeats}
		}
	} else {
		lic, err = s.snipe.FindOrCreateLicense(ctx, s.config.LicenseName, targetSeats, s.config.LicenseCategoryID, manufacturerID, s.config.SupplierID)
		if err != nil {
			return result, err
		}
	}
	slog.Info("license resolved", "id", lic.ID, "seats", lic.Seats, "free", lic.FreeSeatsCount)

	// 7. Expand seats if needed (never shrink automatically).
	if targetSeats > lic.Seats {
		slog.Info("expanding license seats", "current", lic.Seats, "needed", targetSeats)
		if !s.config.DryRun {
			lic, err = s.snipe.UpdateLicenseSeats(ctx, lic.ID, targetSeats)
			if err != nil {
				return result, err
			}
		}
	}

	// 7.5. Refresh the license so FreeSeatsCount is accurate before ghost detection.
	// Snipe-IT's POST (create) response returns free_seats_count: 0 regardless of
	// seat count; a fresh GET gives the real value. Without this, ghost cleanup
	// computes ghostCount = seats - 0 = N and drains all free seats before checkout.
	if !s.config.DryRun && lic.ID != 0 {
		lic, err = s.snipe.FindLicenseByID(ctx, lic.ID)
		if err != nil {
			return result, fmt.Errorf("refreshing license: %w", err)
		}
		slog.Debug("license refreshed", "id", lic.ID, "seats", lic.Seats, "free", lic.FreeSeatsCount)
	}

	// 8. Load current seat assignments.
	// Dry-run with a synthetic license (id=0) skips the API call.
	// In production, id=0 means something went wrong — fail fast.
	checkedOutByEmail := make(map[string]*snipeit.LicenseSeat)
	var freeSeats []*snipeit.LicenseSeat
	if lic.ID != 0 {
		slog.Info("loading current seat assignments")
		seats, err := s.snipe.ListLicenseSeats(ctx, lic.ID)
		if err != nil {
			return result, err
		}
		for i := range seats {
			seat := &seats[i]
			if seat.AssignedTo != nil && seat.AssignedTo.Email != "" {
				checkedOutByEmail[strings.ToLower(seat.AssignedTo.Email)] = seat
			} else {
				freeSeats = append(freeSeats, seat)
			}
		}
	} else if !s.config.DryRun {
		return result, fmt.Errorf("license resolved with id=0 in production mode — check Snipe-IT API permissions and required fields")
	} else {
		slog.Info("[dry-run] skipping seat load for new license")
	}
	slog.Info("seat state loaded", "checked_out", len(checkedOutByEmail), "free", len(freeSeats))

	// 9. Checkout / update loop.
	// TODO: replace `activeUsers` with `usersWithMeta` once step 4 is implemented,
	// and update the notes call to pass the per-user metadata.
	for _, u := range activeUsers {
		email := emailKey(u)
		notes := buildNotes(u) // TODO: pass metadata when step 4 is implemented

		snipeUser, err := s.snipe.FindUserByEmail(ctx, email)
		if err != nil {
			slog.Warn("error looking up Snipe-IT user", "email", email, "error", err)
			result.Warnings++
			continue
		}
		if snipeUser == nil {
			if !s.config.CreateUsers {
				slog.Warn("no Snipe-IT user found for source user", "email", email)
				result.UnmatchedEmails = append(result.UnmatchedEmails, email)
				result.Warnings++
				continue
			}
			// TODO: derive first/last name from vendor user fields.
			// Example: firstName, lastName := splitName(u.DisplayName, email)
			if s.config.DryRun {
				slog.Info("[dry-run] would create Snipe-IT user", "email", email)
				result.UsersCreated++
				result.CheckedOut++
				continue
			}
			// TODO: call s.snipe.CreateUser with appropriate fields.
			// created, err := s.snipe.CreateUser(ctx, firstName, lastName, email, email, "", "")
			// if err != nil {
			//     slog.Warn("failed to create Snipe-IT user", "email", email, "error", err)
			//     result.Warnings++
			//     continue
			// }
			// snipeUser = created
			// result.UsersCreated++
		}

		if existing, ok := checkedOutByEmail[email]; ok {
			if existing.Notes == notes && !s.config.Force {
				slog.Debug("seat up to date", "email", email)
				result.Skipped++
				continue
			}
			slog.Info("updating seat notes", "email", email, "dry_run", s.config.DryRun)
			if !s.config.DryRun {
				if err := s.snipe.UpdateSeatNotes(ctx, lic.ID, existing.ID, notes); err != nil {
					slog.Warn("failed to update seat notes", "email", email, "error", err)
					result.Warnings++
					continue
				}
			}
			result.NotesUpdated++
			continue
		}

		if s.config.DryRun {
			slog.Info("[dry-run] would check out seat", "email", email, "notes", notes)
			result.CheckedOut++
			continue
		}
		if len(freeSeats) == 0 {
			slog.Warn("no free seats available", "email", email)
			result.Warnings++
			continue
		}
		seat := freeSeats[0]
		freeSeats = freeSeats[1:]

		slog.Info("checking out seat", "email", email, "seat_id", seat.ID)
		if err := s.snipe.CheckoutSeat(ctx, lic.ID, seat.ID, snipeUser.ID, notes); err != nil {
			slog.Warn("failed to checkout seat", "email", email, "error", err)
			freeSeats = append(freeSeats, seat) // return on failure
			result.Warnings++
			continue
		}
		result.CheckedOut++
	}

	// 10. Checkin loop — skip when --email filter is set.
	if emailFilter == "" {
		for email, seat := range checkedOutByEmail {
			if _, active := activeEmails[email]; active {
				continue
			}
			slog.Info("checking in seat for inactive user", "email", email, "seat_id", seat.ID, "dry_run", s.config.DryRun)
			if !s.config.DryRun {
				if err := s.snipe.CheckinSeat(ctx, lic.ID, seat.ID); err != nil {
					slog.Warn("failed to checkin seat", "email", email, "error", err)
					result.Warnings++
					continue
				}
			}
			result.CheckedIn++
		}
	}

	return result, nil
}

// emailKey returns the canonical (lowercased) email for a user.
// TODO: update to match your vendor's user struct fields.
// Prefer the primary email field; fall back to username/login.
func emailKey(u vendor.User) string { // TODO: replace vendor.User with your type
	if u.Email != "" {
		return strings.ToLower(u.Email)
	}
	return strings.ToLower(u.Username)
}

// buildNotes returns the formatted notes string written to the Snipe-IT seat.
// TODO: replace with your vendor's metadata. Sort multi-value fields alphabetically.
// Return an empty string if there is nothing to record.
func buildNotes(u vendor.User) string { // TODO: update signature to accept metadata
	return ""
}
```
