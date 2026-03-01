package cmd

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newOrderModifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Modify an existing order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")                     //nolint:errcheck // known flag
			oidStr, _ := cmd.Flags().GetString("oid")                    //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")                     //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price")                //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")                  //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")                   //nolint:errcheck // known flag
			reduce, _ := cmd.Flags().GetBool("reduce")                   //nolint:errcheck // known flag
			vault, _ := cmd.Flags().GetString("vault")                   //nolint:errcheck // known flag
			expiresAfterStr, _ := cmd.Flags().GetString("expires-after") //nolint:errcheck // known flag

			side = strings.ToLower(side)
			if side != "buy" && side != "sell" {
				return output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
					WithDetails("value", side)
			}

			oid, err := strconv.ParseUint(oidStr, 10, 64)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid OID: must be numeric").
					WithDetails("value", oidStr)
			}

			wireTif, ok := tifMap[strings.ToLower(tifFlag)]
			if !ok {
				return output.NewCLIError(output.ErrValidation, "invalid tif: "+tifFlag).
					WithDetails("value", tifFlag).
					WithDetails("valid", "gtc, ioc, alo")
			}

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

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.ModifyOrder(cmd.Context(), exchange.ModifyOrderInput{
				Coin:         coin,
				Oid:          oid,
				Side:         side,
				Price:        price,
				Size:         size,
				Tif:          wireTif,
				ReduceOnly:   reduce,
				ExpiresAfter: expiresAfter,
				VaultAddr:    vault,
				DryRun:       cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("oid", "", "order ID to modify")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("price", "", "new limit price")
	cmd.Flags().String("size", "", "new order size")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().Bool("reduce", false, "reduce-only order")
	cmd.Flags().String("vault", "", "vault address")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	for _, required := range []string{"coin", "oid", "side", "price", "size"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
