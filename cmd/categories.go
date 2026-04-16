package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jackvaughanjr/2snipe-manager/internal/snipeit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Manage Snipe-IT license categories",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all license categories in Snipe-IT",
	RunE:  runCategoriesList,
}

var categoriesSeedDryRun bool

var categoriesSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed default license categories into Snipe-IT (idempotent)",
	Long: `Ensure all default license categories exist in Snipe-IT. Categories that
already exist are skipped silently. Safe to run multiple times.`,
	RunE: runCategoriesSeed,
}

func init() {
	rootCmd.AddCommand(categoriesCmd)
	categoriesCmd.AddCommand(categoriesListCmd)
	categoriesCmd.AddCommand(categoriesSeedCmd)
	categoriesSeedCmd.Flags().BoolVar(&categoriesSeedDryRun, "dry-run", false,
		"show what would be created without making API changes")
}

// snipeitClient constructs a Snipe-IT client from snipemgr.yaml credentials.
func snipeitClient() (*snipeit.Client, error) {
	url := viper.GetString("snipe_it.url")
	key := viper.GetString("snipe_it.api_key")
	if url == "" {
		return nil, fmt.Errorf("snipe_it.url is not set in snipemgr.yaml")
	}
	if key == "" {
		return nil, fmt.Errorf("snipe_it.api_key is not set in snipemgr.yaml")
	}
	return snipeit.NewClient(url, key), nil
}

func runCategoriesList(_ *cobra.Command, _ []string) error {
	client, err := snipeitClient()
	if err != nil {
		return fatal("Snipe-IT credentials not configured: %v", err)
	}

	cats, err := client.ListCategories()
	if err != nil {
		return fatal("listing categories: %v", err)
	}

	if len(cats) == 0 {
		fmt.Println("No license categories found in Snipe-IT.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME")
	for _, c := range cats {
		fmt.Fprintf(w, "%d\t%s\n", c.ID, c.Name)
	}
	_ = w.Flush()
	return nil
}

func runCategoriesSeed(_ *cobra.Command, _ []string) error {
	client, err := snipeitClient()
	if err != nil {
		return fatal("Snipe-IT credentials not configured: %v", err)
	}

	if categoriesSeedDryRun {
		return runCategoriesSeedDryRun(client)
	}

	// Fetch existing categories once to avoid redundant GET calls per category.
	existing, err := client.ListCategories()
	if err != nil {
		return fatal("listing categories: %v", err)
	}
	existingNames := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingNames[strings.ToLower(c.Name)] = true
	}

	created, failed := 0, 0
	for _, name := range snipeit.DefaultCategories {
		if existingNames[strings.ToLower(name)] {
			continue // already exists — skip silently
		}
		id, err := client.CreateCategory(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %q: %v\n", name, err)
			failed++
			continue
		}
		fmt.Printf("  ✓ %s (id=%d)\n", name, id)
		created++
	}

	switch {
	case created == 0 && failed == 0:
		fmt.Println("All default categories already exist — nothing to do.")
	case failed == 0:
		fmt.Printf("Done. Created %d categories.\n", created)
	default:
		fmt.Printf("Done. Created %d, %d failed (see warnings above).\n", created, failed)
	}
	return nil
}

func runCategoriesSeedDryRun(client *snipeit.Client) error {
	cats, err := client.ListCategories()
	if err != nil {
		return fatal("listing categories: %v", err)
	}

	existing := make(map[string]bool, len(cats))
	for _, c := range cats {
		existing[strings.ToLower(c.Name)] = true
	}

	fmt.Println("[dry-run] Default categories:")
	for _, name := range snipeit.DefaultCategories {
		if existing[strings.ToLower(name)] {
			fmt.Printf("  [exists]        %s\n", name)
		} else {
			fmt.Printf("  [would create]  %s\n", name)
		}
	}
	return nil
}
