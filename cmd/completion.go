package cmd

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/jackvaughanjr/2snipe-manager/internal/state"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var completionCmd = &cobra.Command{
	Use:                   "completion [bash|zsh|fish|powershell]",
	Short:                 "Generate shell completion scripts",
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	DisableFlagsInUseLine: true,
	Long: `Generate shell completion scripts for snipemgr.

To load completions:

Bash:

  Linux:
    snipemgr completion bash > /etc/bash_completion.d/snipemgr

  macOS:
    snipemgr completion bash > "$(brew --prefix)/etc/bash_completion.d/snipemgr"

Zsh:
  mkdir -p "${fpath[1]}"
  snipemgr completion zsh > "${fpath[1]}/_snipemgr"

Fish:
  mkdir -p ~/.config/fish/completions
  snipemgr completion fish > ~/.config/fish/completions/snipemgr.fish

PowerShell:
  snipemgr completion powershell | Out-String | Invoke-Expression

To configure your shell to load completions for each session, see your shell's
documentation for completion support.`,
	RunE: runCompletion,
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

func runCompletion(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell %q", args[0])
	}
}

func integrationNameCompletion(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = "~/.snipemgr/state.json"
	}

	s, err := state.ReadState(statePath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := slices.Sorted(maps.Keys(s.Integrations))
	if toComplete == "" {
		return names, cobra.ShellCompDirectiveNoFileComp
	}

	matches := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}
