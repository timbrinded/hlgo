package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"github.com/timbrinded/hlgo/pkg/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "config",
		Short:       "Configure hlgo settings and credentials",
		Annotations: map[string]string{"skipConfig": "true"},
		Long: `Initialize configuration, display resolved settings (with key redaction),
and test wallet connectivity. Configuration is
stored at ~/.hlgo/config.yaml by default.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newConfigInitCmd(),
		newConfigShowCmd(),
		newConfigTestCmd(),
	)

	return cmd
}

// configFileData is the structure written to the YAML config file.
// Fields mirror the persisted fields in config.Config — keep in sync.
type configFileData struct {
	PrivateKeyEnv  string `yaml:"private_key_env"`
	AccountAddress string `yaml:"account_address"`
	DefaultDex     string `yaml:"default_dex"`
	MetadataTTL    int    `yaml:"metadata_ttl"`
}

// resolveConfigPath expands the default sentinel path to an absolute path.
func resolveConfigPath(flagValue string) (string, error) {
	if flagValue != config.DefaultConfigPath {
		return flagValue, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".hlgo", "config.yaml"), nil
}

func newConfigInitCmd() *cobra.Command {
	var (
		privateKeyEnv  string
		accountAddress string
		defaultDex     string
		metadataTTL    int
		force          bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new hlgo configuration file",
		Long: `Creates a config file with the specified settings. All flags have sensible
defaults — bare "hlgo config init" creates a valid config. Will not overwrite
an existing config unless --force is passed.`,
		Annotations: map[string]string{"skipConfig": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(cmd.Flag("config").Value.String())
			if err != nil {
				return err
			}

			if !force {
				if _, err := os.Stat(cfgPath); err == nil {
					return fmt.Errorf("config file already exists: %s (use --force to overwrite)", cfgPath)
				}
			}

			data := configFileData{
				PrivateKeyEnv:  privateKeyEnv,
				AccountAddress: accountAddress,
				DefaultDex:     defaultDex,
				MetadataTTL:    metadataTTL,
			}

			out, err := yaml.Marshal(data)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}

			if err := os.WriteFile(cfgPath, out, 0600); err != nil {
				return fmt.Errorf("write config file: %w", err)
			}

			if privateKeyEnv != "" && os.Getenv(privateKeyEnv) == "" {
				if _, werr := fmt.Fprintf(cmd.ErrOrStderr(), "warning: environment variable %s is not set\n", privateKeyEnv); werr != nil {
					return werr
				}
			}

			result, err := json.Marshal(map[string]string{"path": cfgPath})
			if err != nil {
				return fmt.Errorf("marshal result: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(result))
			return err
		},
	}

	cmd.Flags().StringVar(&privateKeyEnv, "private-key-env", "HL_PRIVATE_KEY", "env var name for private key")
	cmd.Flags().StringVar(&accountAddress, "account-address", "", "default account address for reads and account-context commands")
	cmd.Flags().StringVar(&defaultDex, "default-dex", "", "default HIP-3 dex name")
	cmd.Flags().IntVar(&metadataTTL, "metadata-ttl", 300, "metadata cache TTL in seconds")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if accountAddress == "" {
			return nil
		}
		if !common.IsHexAddress(accountAddress) {
			return fmt.Errorf("invalid --account-address: must be 0x-prefixed 40-hex address")
		}
		return nil
	}

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Display resolved configuration with key redaction",
		Annotations: map[string]string{"skipConfig": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := newShowViper(cmd)
			cfg, err := config.Load(v)
			if err != nil {
				return err
			}

			privateKeyVal := os.Getenv(cfg.PrivateKeyEnv)

			result := map[string]any{
				"config_file":     v.ConfigFileUsed(),
				"private_key_env": cfg.PrivateKeyEnv,
				"private_key_set": privateKeyVal != "",
				"account_address": cfg.AccountAddress,
				"testnet":         cfg.Testnet,
				"format":          cfg.Format,
				"dex":             cfg.Dex,
				"default_dex":     cfg.DefaultDex,
				"metadata_ttl":    cfg.MetadataTTL,
			}

			if privateKeyVal != "" {
				result["private_key_preview"] = config.RedactKey(privateKeyVal)
			}

			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return err
		},
	}
}

func newConfigTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "test",
		Short:       "Validate configuration and connectivity",
		Annotations: map[string]string{"skipConfig": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := newShowViper(cmd)
			cfg, err := config.Load(v)

			configFile := v.ConfigFileUsed()
			result := map[string]any{
				"config_file":         configFile,
				"config_readable":     configFile != "" && err == nil,
				"private_key_env_set": false,
			}

			if err != nil {
				result["config_error"] = err.Error()
				result["connectivity"] = map[string]string{
					"status": "skipped",
					"reason": "config not readable",
				}
				if out, merr := json.MarshalIndent(result, "", "  "); merr == nil {
					if _, werr := fmt.Fprintln(cmd.OutOrStdout(), string(out)); werr != nil {
						return werr
					}
				}
				return fmt.Errorf("config not readable: %w", err)
			}

			privateKeySet := cfg.PrivateKeyEnv != "" && os.Getenv(cfg.PrivateKeyEnv) != ""
			result["private_key_env_set"] = privateKeySet

			// Test connectivity by fetching mid prices.
			ic := buildInfoClient(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			raw, cerr := ic.AllMids(ctx, cfg.Dex)
			if cerr != nil {
				result["connectivity"] = map[string]string{
					"status": "failed",
					"error":  cerr.Error(),
				}
			} else {
				// Count coins to give useful feedback.
				var mids map[string]string
				if jerr := json.Unmarshal(raw, &mids); jerr == nil {
					result["connectivity"] = map[string]any{
						"status": "ok",
						"coins":  len(mids),
					}
				} else {
					result["connectivity"] = map[string]string{
						"status": "ok",
					}
				}
			}

			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), string(out)); err != nil {
				return err
			}

			// Return an error for missing private key so scripts can check the exit
			// code. JSON is already written to stdout; SilenceErrors prevents
			// Cobra from writing the error to stderr.
			if !privateKeySet {
				return fmt.Errorf("private key env var %q is not set", cfg.PrivateKeyEnv)
			}
			return nil
		},
	}
}

// newShowViper creates a fresh viper instance with flag values from cmd,
// for use by config show and config test (which skip PersistentPreRunE).
// String flags are filtered for non-empty to avoid overriding file/default
// values; "config" always passes since it has a non-empty default.
func newShowViper(cmd *cobra.Command) *viper.Viper {
	v := viper.New()

	for _, key := range []string{"config", "format", "dex"} {
		if val := cmd.Flag(key).Value.String(); val != "" {
			v.Set(key, val)
		}
	}
	for _, key := range []string{"testnet", "dry-run", "quiet"} {
		v.Set(key, cmd.Flag(key).Value.String() == "true")
	}

	return v
}
