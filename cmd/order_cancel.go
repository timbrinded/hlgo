package cmd

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
)

func newOrderCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an order by OID or CLOID",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")                     //nolint:errcheck // known flag
			oidStr, _ := cmd.Flags().GetString("oid")                    //nolint:errcheck // known flag
			cloidStr, _ := cmd.Flags().GetString("cloid")                //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of")       //nolint:errcheck // known flag
			expiresAfterStr, _ := cmd.Flags().GetString("expires-after") //nolint:errcheck // known flag

			// Mutual exclusion: exactly one of --oid or --cloid.
			if oidStr == "" && cloidStr == "" {
				return output.NewCLIError(output.ErrValidation, "one of --oid or --cloid is required")
			}
			if oidStr != "" && cloidStr != "" {
				return output.NewCLIError(output.ErrValidation, "--oid and --cloid are mutually exclusive")
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
				return output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
					WithDetails("on_behalf_of", onBehalfOf)
			}

			var expiresAfter *int64
			if expiresAfterStr != "" {
				ms, err := parseTimeFlag(expiresAfterStr)
				if err != nil {
					return err
				}
				if ms <= 0 {
					return output.NewCLIError(output.ErrValidation, "expires-after must be a positive Unix ms timestamp")
				}
				expiresAfter = &ms
			}

			if cloidStr != "" {
				// Cancel by CLOID — resolve coin to asset ID.
				assetID, err := resolveAssetID(cmd, cfg, coin)
				if err != nil {
					return err
				}

				result, err := exec.CancelByCloid(cmd.Context(), []exchange.CancelByCloidWire{
					{Asset: assetID, Cloid: cloidStr},
				}, onBehalfOf, cfg.DryRun, expiresAfter)
				if err != nil {
					return err
				}
				return printResult(cmd, cfg, result, nil)
			}

			// Cancel by OID.
			oid, err := strconv.ParseUint(oidStr, 10, 64)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid OID: must be numeric").
					WithDetails("value", oidStr)
			}

			assetID, err := resolveAssetID(cmd, cfg, coin)
			if err != nil {
				return err
			}

			result, err := exec.CancelOrders(cmd.Context(), []exchange.CancelWire{
				{A: assetID, O: oid},
			}, onBehalfOf, cfg.DryRun, expiresAfter)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (required for asset ID resolution)")
	cmd.Flags().String("oid", "", "order ID to cancel")
	cmd.Flags().String("cloid", "", "client order ID to cancel")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	//nolint:errcheck // MarkFlagRequired on known flags never fails
	cmd.MarkFlagRequired("coin")

	return cmd
}

func newOrderCancelAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel-all",
		Short: "Cancel all open orders (optionally for a specific coin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")                     //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of")       //nolint:errcheck // known flag
			expiresAfterStr, _ := cmd.Flags().GetString("expires-after") //nolint:errcheck // known flag

			if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
				return output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
					WithDetails("on_behalf_of", onBehalfOf)
			}

			// Resolve user address for fetching open orders.
			addr := onBehalfOf
			if addr == "" {
				var err error
				addr, err = info.ResolveUserAddress("", cfg)
				if err != nil {
					return err
				}
			}

			// Fetch all open orders.
			ic := buildInfoClient(cfg)
			raw, err := ic.FrontendOpenOrders(cmd.Context(), addr, cfg.Dex)
			if err != nil {
				return err
			}

			orders, err := info.ParseOpenOrdersResult(raw)
			if err != nil {
				return err
			}

			if len(orders) == 0 {
				return printResult(cmd, cfg, mustMarshal(map[string]string{
					"status": "ok", "message": "no open orders to cancel",
				}), nil)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			var expiresAfter *int64
			if expiresAfterStr != "" {
				ms, err := parseTimeFlag(expiresAfterStr)
				if err != nil {
					return err
				}
				if ms <= 0 {
					return output.NewCLIError(output.ErrValidation, "expires-after must be a positive Unix ms timestamp")
				}
				expiresAfter = &ms
			}

			// Build cancel list, optionally filtered by coin.
			var cancels []exchange.CancelWire
			for _, o := range orders {
				if coin != "" && o.Coin != coin {
					continue
				}
				if o.Oid < 0 {
					return output.NewCLIError(output.ErrAPI, "open order returned negative OID").
						WithDetails("coin", o.Coin).
						WithDetails("oid", o.Oid)
				}
				assetID, err := resolveAssetID(cmd, cfg, o.Coin)
				if err != nil {
					return err
				}
				cancels = append(cancels, exchange.CancelWire{A: assetID, O: uint64(o.Oid)})
			}

			if len(cancels) == 0 {
				return printResult(cmd, cfg, mustMarshal(map[string]string{
					"status": "ok", "message": "no matching orders to cancel",
				}), nil)
			}

			result, err := exec.CancelOrders(cmd.Context(), cancels, onBehalfOf, cfg.DryRun, expiresAfter)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "only cancel orders for this coin")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	return cmd
}

// resolveAssetID resolves a coin name to its integer asset ID.
func resolveAssetID(cmd *cobra.Command, cfg *config.Config, coin string) (int, error) {
	c := buildHTTPClient(cfg)
	cacheDir := resolveCacheDir(cfg)
	r := resolver.NewResolver(c, cacheDir, 0)
	info, err := r.ResolveAsset(cmd.Context(), coin)
	if err != nil {
		return 0, err
	}
	return info.AssetID, nil
}

// buildHTTPClient creates a raw HTTP client for the current config.
func buildHTTPClient(cfg *config.Config) *client.Client {
	return client.NewClient(baseURL(cfg))
}
