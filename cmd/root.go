package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timbrinded/hlgo/pkg/config"
)

// NewRootCommand constructs the root hlgo command with all subcommands registered.
// Version is injected at build time via ldflags — no global mutable state.
func NewRootCommand(version string) *cobra.Command {
	v := viper.New()

	root := &cobra.Command{
		Use:   "hlgo",
		Short: "Hyperliquid CLI for AI agents",
		Long: `hlgo is a command-line tool for the Hyperliquid exchange, designed for
AI agent consumption. All commands return structured JSON to stdout by default.
Errors are returned as structured JSON to stderr with machine-readable codes.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Annotations["skipConfig"] == "true" {
				return nil
			}
			// Cobra built-in commands (help, completion) lack annotations;
			// skip config loading for them by name.
			switch cmd.Name() {
			case "help", "completion":
				return nil
			}
			cfg, err := config.Load(v)
			if err != nil {
				return err
			}
			cmd.SetContext(config.WithContext(cmd.Context(), cfg))
			return nil
		},
	}

	registerPersistentFlags(root)
	bindEnvVars(v, root)

	root.AddCommand(
		newVersionCmd(version),
		newInfoCmd(),
		newOrderCmd(),
		newPositionCmd(),
		newAgentCmd(),
		newAccountCmd(),
		newConfigCmd(),
	)

	return root
}

// registerPersistentFlags adds global flags available to all commands.
func registerPersistentFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.String("format", "json", "output format: json, table, csv")
	flags.Bool("testnet", false, "use testnet endpoints")
	flags.String("config", "~/.hlgo/config.yaml", "config file path")
	flags.Bool("quiet", false, "suppress non-essential output")
	flags.Bool("dry-run", false, "show what would be signed/sent without executing")
	flags.String("dex", "", "HIP-3 perp dex name (empty = validator perps)")
}

// bindEnvVars binds environment variables to flags via viper.
// Only flags with documented env var support are bound — quiet and dry-run are
// intentionally flag-only to prevent accidental activation via environment.
// Panics on binding errors since they indicate programmer mistakes (wrong flag names).
func bindEnvVars(v *viper.Viper, cmd *cobra.Command) {
	v.SetEnvPrefix("HL")

	bindings := map[string]string{
		"format":  "FORMAT",
		"testnet": "TESTNET",
		"config":  "CONFIG",
		"dex":     "DEX",
	}

	for flag, env := range bindings {
		if err := v.BindEnv(flag, fmt.Sprintf("HL_%s", env)); err != nil {
			panic(fmt.Sprintf("bind env %s: %v", env, err))
		}
		if err := v.BindPFlag(flag, cmd.PersistentFlags().Lookup(flag)); err != nil {
			panic(fmt.Sprintf("bind flag %s: %v", flag, err))
		}
	}

	// Flag-only bindings (no env var) — quiet and dry-run are intentionally
	// not activatable via environment to prevent accidental activation.
	for _, flag := range []string{"quiet", "dry-run"} {
		if err := v.BindPFlag(flag, cmd.PersistentFlags().Lookup(flag)); err != nil {
			panic(fmt.Sprintf("bind flag %s: %v", flag, err))
		}
	}
}
