package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
)

// tifMap maps user-friendly TIF strings to wire-format values.
var tifMap = map[string]string{
	"gtc": "Gtc",
	"ioc": "Ioc",
	"alo": "Alo",
}

func newOrderPlaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place a limit order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")      //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")      //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price") //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")   //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")    //nolint:errcheck // known flag
			reduce, _ := cmd.Flags().GetBool("reduce")    //nolint:errcheck // known flag
			cloidStr, _ := cmd.Flags().GetString("cloid") //nolint:errcheck // known flag
			vault, _ := cmd.Flags().GetString("vault")    //nolint:errcheck // known flag

			// Validate side.
			side = strings.ToLower(side)
			if side != "buy" && side != "sell" {
				return output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
					WithDetails("value", side)
			}

			// Map TIF.
			wireTif, ok := tifMap[strings.ToLower(tifFlag)]
			if !ok {
				return output.NewCLIError(output.ErrValidation, "invalid tif: "+tifFlag).
					WithDetails("value", tifFlag).
					WithDetails("valid", "gtc, ioc, alo")
			}

			// Parse price and size with decimal precision.
			price, err := decimal.NewFromString(priceStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid price").
					WithDetails("value", priceStr)
			}
			size, err := decimal.NewFromString(sizeStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid size").
					WithDetails("value", sizeStr)
			}

			var cloid *string
			if cloidStr != "" {
				cloid = &cloidStr
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.PlaceOrder(cmd.Context(), exchange.PlaceOrderInput{
				Coin:       coin,
				Side:       side,
				Price:      price,
				Size:       size,
				Tif:        wireTif,
				ReduceOnly: reduce,
				Cloid:      cloid,
				VaultAddr:  vault,
				DryRun:     cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("price", "", "limit price")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().Bool("reduce", false, "reduce-only order")
	cmd.Flags().String("cloid", "", "client order ID")
	cmd.Flags().String("vault", "", "vault address")

	for _, required := range []string{"coin", "side", "price", "size"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}

// buildExecutor constructs an exchange.Executor from the current config.
func buildExecutor(cfg *config.Config) (*exchange.Executor, error) {
	keyHex := os.Getenv(cfg.AgentKeyEnv)
	if keyHex == "" {
		return nil, output.NewCLIError(output.ErrConfig, "agent key not set").
			WithDetails("env_var", cfg.AgentKeyEnv)
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

// resolveCacheDir returns the cache directory path for the current network.
func resolveCacheDir(cfg *config.Config) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	network := "mainnet"
	if cfg.Testnet {
		network = "testnet"
	}
	return home + "/.hlgo/cache/" + network
}
