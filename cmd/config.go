package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
and test wallet connectivity and agent approval status. Configuration is
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
type configFileData struct {
	AgentKeyEnv  string `yaml:"agent_key_env"`
	MasterKeyEnv string `yaml:"master_key_env"`
	DefaultDex   string `yaml:"default_dex"`
	MetadataTTL  int    `yaml:"metadata_ttl"`
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
		agentKeyEnv  string
		masterKeyEnv string
		defaultDex   string
		metadataTTL  int
		force        bool
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
				AgentKeyEnv:  agentKeyEnv,
				MasterKeyEnv: masterKeyEnv,
				DefaultDex:   defaultDex,
				MetadataTTL:  metadataTTL,
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

			for _, envName := range []string{agentKeyEnv, masterKeyEnv} {
				if envName != "" && os.Getenv(envName) == "" {
					if _, werr := fmt.Fprintf(cmd.ErrOrStderr(), "warning: environment variable %s is not set\n", envName); werr != nil {
						return werr
					}
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

	cmd.Flags().StringVar(&agentKeyEnv, "agent-key-env", "HL_AGENT_KEY", "env var name for agent private key")
	cmd.Flags().StringVar(&masterKeyEnv, "master-key-env", "HL_MASTER_KEY", "env var name for master private key")
	cmd.Flags().StringVar(&defaultDex, "default-dex", "", "default HIP-3 dex name")
	cmd.Flags().IntVar(&metadataTTL, "metadata-ttl", 300, "metadata cache TTL in seconds")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")

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

			agentKeyVal := os.Getenv(cfg.AgentKeyEnv)
			masterKeyVal := os.Getenv(cfg.MasterKeyEnv)

			result := map[string]any{
				"config_file":    v.ConfigFileUsed(),
				"agent_key_env":  cfg.AgentKeyEnv,
				"agent_key_set":  agentKeyVal != "",
				"master_key_env": cfg.MasterKeyEnv,
				"master_key_set": masterKeyVal != "",
				"testnet":        cfg.Testnet,
				"format":         cfg.Format,
				"default_dex":    cfg.DefaultDex,
				"metadata_ttl":   cfg.MetadataTTL,
			}

			if agentKeyVal != "" {
				result["agent_key_preview"] = config.RedactKey(agentKeyVal)
			}
			if masterKeyVal != "" {
				result["master_key_preview"] = config.RedactKey(masterKeyVal)
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

			result := map[string]any{
				"config_readable":    err == nil,
				"agent_key_env_set":  false,
				"master_key_env_set": false,
				"connectivity": map[string]string{
					"status": "skipped",
					"reason": "http client not implemented",
				},
				"agent_approved": map[string]string{
					"status": "skipped",
					"reason": "http client not implemented",
				},
			}

			if err != nil {
				result["config_error"] = err.Error()
				if out, merr := json.MarshalIndent(result, "", "  "); merr == nil {
					if _, werr := fmt.Fprintln(cmd.OutOrStdout(), string(out)); werr != nil {
						return werr
					}
				}
				return fmt.Errorf("config not readable: %w", err)
			}

			agentKeySet := cfg.AgentKeyEnv != "" && os.Getenv(cfg.AgentKeyEnv) != ""
			masterKeySet := cfg.MasterKeyEnv != "" && os.Getenv(cfg.MasterKeyEnv) != ""
			result["agent_key_env_set"] = agentKeySet
			result["master_key_env_set"] = masterKeySet

			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), string(out)); err != nil {
				return err
			}

			if !agentKeySet {
				return fmt.Errorf("agent key env var %q is not set", cfg.AgentKeyEnv)
			}
			return nil
		},
	}
}

// newShowViper creates a fresh viper instance with flag values from cmd,
// for use by config show and config test (which skip PersistentPreRunE).
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
