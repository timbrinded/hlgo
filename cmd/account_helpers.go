package cmd

import (
	"os"
	"time"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
)

// buildMasterExecutor constructs an exchange.Executor signed by the master wallet.
func buildMasterExecutor(cfg *config.Config) (*exchange.Executor, error) {
	keyHex := os.Getenv(cfg.MasterKeyEnv)
	if keyHex == "" {
		return nil, output.NewCLIError(output.ErrConfig, "master key not set").
			WithDetails("env_var", cfg.MasterKeyEnv)
	}

	s, err := signer.NewSigner(keyHex)
	if err != nil {
		return nil, err
	}

	c := client.NewClient(baseURL(cfg))
	cacheDir := resolveCacheDir(cfg)
	r := resolver.NewResolver(c, cacheDir, time.Duration(cfg.MetadataTTL)*time.Second)

	return exchange.NewExecutor(s, c, r, !cfg.Testnet), nil
}

func requireConfirm(cmdName string, confirmed bool, dryRun bool) error {
	if confirmed || dryRun {
		return nil
	}
	return output.NewCLIError(output.ErrValidation, cmdName+" requires --confirm (or --yes)").
		WithDetails("hint", "add --confirm to execute, or --dry-run to preview")
}
