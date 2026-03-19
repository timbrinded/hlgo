package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newOrderCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an order by OID or CLOID",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")   //nolint:errcheck // known flag
			oidStr, _ := cmd.Flags().GetString("oid")  //nolint:errcheck // known flag
			cloid, _ := cmd.Flags().GetString("cloid") //nolint:errcheck // known flag

			if oidStr == "" && cloid == "" {
				return output.NewCLIError(output.ErrValidation, "one of --oid or --cloid is required")
			}
			if oidStr != "" && cloid != "" {
				return output.NewCLIError(output.ErrValidation, "--oid and --cloid are mutually exclusive")
			}

			expiresAfter, err := parseOptionalExpiresAfter(cmd)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			assetInfo, err := buildResolver(cfg).ResolveAsset(cmd.Context(), coin)
			if err != nil {
				return err
			}

			var result []byte
			if cloid != "" {
				result, err = exec.CancelByCloid(cmd.Context(), []exchange.CancelByCloidWire{{Asset: assetInfo.AssetID, Cloid: cloid}}, cfg.DryRun, expiresAfter)
			} else {
				oid, parseErr := strconv.ParseUint(oidStr, 10, 64)
				if parseErr != nil {
					return output.NewCLIError(output.ErrValidation, "invalid OID: must be numeric").
						WithDetails("value", oidStr)
				}
				result, err = exec.CancelOrders(cmd.Context(), []exchange.CancelWire{{A: assetInfo.AssetID, O: oid}}, cfg.DryRun, expiresAfter)
			}
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (required for asset ID resolution)")
	cmd.Flags().String("oid", "", "order ID to cancel")
	cmd.Flags().String("cloid", "", "client order ID to cancel")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	mustMarkRequiredFlags(cmd, "coin")

	return cmd
}

func newOrderCancelAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel-all",
		Short: "Cancel all open orders (optionally for a specific coin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")               //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of") //nolint:errcheck // known flag

			addr, err := info.ResolveUserAddress(onBehalfOf, cfg)
			if err != nil {
				return err
			}

			_, orders, err := fetchOpenOrders(cmd, cfg, addr, cfg.Dex)
			if err != nil {
				return err
			} else if len(orders) == 0 {
				return printOKMessage(cmd, cfg, "no open orders to cancel")
			}

			cancels := make([]exchange.CancelWire, 0, len(orders))
			for _, order := range orders {
				if coin != "" && order.Coin != coin {
					continue
				}
				cancel, err := cancelWireForOpenOrder(cmd, cfg, order)
				if err != nil {
					return err
				}
				cancels = append(cancels, cancel)
			}
			if len(cancels) == 0 {
				return printOKMessage(cmd, cfg, "no matching orders to cancel")
			}

			expiresAfter, err := parseOptionalExpiresAfter(cmd)
			if err != nil {
				return err
			}
			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.CancelOrders(cmd.Context(), cancels, cfg.DryRun, expiresAfter)
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

func cancelWireForOpenOrder(cmd *cobra.Command, cfg *config.Config, order info.OpenOrder) (exchange.CancelWire, error) {
	if order.Oid < 0 {
		return exchange.CancelWire{}, output.NewCLIError(output.ErrAPI, "open order returned negative OID").
			WithDetails("coin", order.Coin).
			WithDetails("oid", order.Oid)
	}

	assetInfo, err := buildResolver(cfg).ResolveAsset(cmd.Context(), order.Coin)
	if err != nil {
		return exchange.CancelWire{}, err
	}
	return exchange.CancelWire{A: assetInfo.AssetID, O: uint64(order.Oid)}, nil
}
