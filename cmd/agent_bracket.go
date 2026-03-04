package cmd

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAgentBracketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bracket",
		Short: "Place entry + TP + SL in one grouped order action",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")                             //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")                             //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price")                        //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")                          //nolint:errcheck // known flag
			tpStr, _ := cmd.Flags().GetString("tp")                              //nolint:errcheck // known flag
			slStr, _ := cmd.Flags().GetString("sl")                              //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")                           //nolint:errcheck // known flag
			cloidStr, _ := cmd.Flags().GetString("cloid")                        //nolint:errcheck // known flag
			builderAddr, _ := cmd.Flags().GetString("builder")                   //nolint:errcheck // known flag
			builderFeeTenthsBp, _ := cmd.Flags().GetInt("builder-fee-tenths-bp") //nolint:errcheck // known flag
			expiresAfterStr, _ := cmd.Flags().GetString("expires-after")         //nolint:errcheck // known flag

			side = strings.ToLower(side)
			if side != "buy" && side != "sell" {
				return output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
					WithDetails("value", side)
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
			tp, err := decimal.NewFromString(tpStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid tp").
					WithDetails("value", tpStr)
			}
			sl, err := decimal.NewFromString(slStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid sl").
					WithDetails("value", slStr)
			}

			wireTif, ok := tifMap[strings.ToLower(tifFlag)]
			if !ok {
				return output.NewCLIError(output.ErrValidation, "invalid tif: "+tifFlag).
					WithDetails("value", tifFlag).
					WithDetails("valid", "gtc, ioc, alo")
			}

			if side == "buy" {
				if !tp.GreaterThan(price) {
					return output.NewCLIError(output.ErrValidation, "for buy brackets, tp must be greater than entry price").
						WithDetails("price", price.String()).
						WithDetails("tp", tp.String())
				}
				if !sl.LessThan(price) {
					return output.NewCLIError(output.ErrValidation, "for buy brackets, sl must be less than entry price").
						WithDetails("price", price.String()).
						WithDetails("sl", sl.String())
				}
			} else {
				if !tp.LessThan(price) {
					return output.NewCLIError(output.ErrValidation, "for sell brackets, tp must be less than entry price").
						WithDetails("price", price.String()).
						WithDetails("tp", tp.String())
				}
				if !sl.GreaterThan(price) {
					return output.NewCLIError(output.ErrValidation, "for sell brackets, sl must be greater than entry price").
						WithDetails("price", price.String()).
						WithDetails("sl", sl.String())
				}
			}

			var cloid *string
			if cloidStr != "" {
				cloid = &cloidStr
			}

			changedBuilder := cmd.Flags().Changed("builder")
			changedBuilderFee := cmd.Flags().Changed("builder-fee-tenths-bp")
			if changedBuilder != changedBuilderFee {
				return output.NewCLIError(output.ErrValidation, "--builder and --builder-fee-tenths-bp must be provided together")
			}

			var builder *exchange.BuilderInfo
			if changedBuilder {
				if !common.IsHexAddress(builderAddr) {
					return output.NewCLIError(output.ErrValidation, "invalid builder address").
						WithDetails("builder", builderAddr)
				}
				if builderFeeTenthsBp < 0 {
					return output.NewCLIError(output.ErrValidation, "builder fee must be non-negative").
						WithDetails("builder_fee_tenths_bp", builderFeeTenthsBp)
				}
				builder = &exchange.BuilderInfo{
					B: strings.ToLower(builderAddr),
					F: builderFeeTenthsBp,
				}
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

			tpTrigger := tp.String()
			slTrigger := sl.String()
			result, err := exec.PlaceOrder(cmd.Context(), exchange.PlaceOrderInput{
				Coin:         coin,
				Side:         side,
				Price:        price,
				Size:         size,
				Tif:          wireTif,
				Cloid:        cloid,
				TpTrigger:    &tpTrigger,
				SlTrigger:    &slTrigger,
				Builder:      builder,
				ExpiresAfter: expiresAfter,
				DryRun:       cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("price", "", "entry limit price")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().String("tp", "", "take-profit trigger price")
	cmd.Flags().String("sl", "", "stop-loss trigger price")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().String("cloid", "", "client order ID")
	cmd.Flags().String("builder", "", "builder address for optional builder fee routing")
	cmd.Flags().Int("builder-fee-tenths-bp", 0, "builder fee in tenths of a basis point (requires --builder)")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	for _, required := range []string{"coin", "side", "price", "size", "tp", "sl"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
