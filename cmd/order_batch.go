package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/wire"
)

// batchEntry is an individual order in a batch file.
type batchEntry struct {
	Coin       string  `json:"coin"`
	Side       string  `json:"side"`
	Price      string  `json:"price"`
	Size       string  `json:"size"`
	Tif        string  `json:"tif"`
	ReduceOnly bool    `json:"reduce_only"`
	Cloid      *string `json:"cloid,omitempty"`
}

func newOrderBatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Place multiple orders from a JSON file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			filePath, _ := cmd.Flags().GetString("file")                         //nolint:errcheck // known flag
			builderAddr, _ := cmd.Flags().GetString("builder")                   //nolint:errcheck // known flag
			builderFeeTenthsBp, _ := cmd.Flags().GetInt("builder-fee-tenths-bp") //nolint:errcheck // known flag
			expiresAfterStr, _ := cmd.Flags().GetString("expires-after")         //nolint:errcheck // known flag

			data, err := os.ReadFile(filePath)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "failed to read batch file").
					WithDetails("path", filePath).
					WithDetails("cause", err.Error())
			}

			var entries []batchEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid batch file JSON").
					WithDetails("path", filePath).
					WithDetails("cause", err.Error())
			}

			if len(entries) == 0 {
				return output.NewCLIError(output.ErrValidation, "batch file contains no orders").
					WithDetails("path", filePath)
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

			// Resolve and validate each entry.
			r := buildBatchResolver(cfg)
			var params []exchange.OrderParams
			for i, e := range entries {
				side := strings.ToLower(e.Side)
				if side != "buy" && side != "sell" {
					return output.NewCLIError(output.ErrValidation, "invalid side in batch entry").
						WithDetails("index", i).
						WithDetails("value", e.Side)
				}

				tifKey := strings.ToLower(e.Tif)
				if tifKey == "" {
					tifKey = "gtc"
				}
				wireTif, ok := tifMap[tifKey]
				if !ok {
					return output.NewCLIError(output.ErrValidation, "invalid tif in batch entry").
						WithDetails("index", i).
						WithDetails("value", e.Tif)
				}

				price, err := decimal.NewFromString(e.Price)
				if err != nil {
					return output.NewCLIError(output.ErrValidation, "invalid price in batch entry").
						WithDetails("index", i).
						WithDetails("value", e.Price)
				}
				size, err := decimal.NewFromString(e.Size)
				if err != nil {
					return output.NewCLIError(output.ErrValidation, "invalid size in batch entry").
						WithDetails("index", i).
						WithDetails("value", e.Size)
				}

				info, err := r.ResolveAsset(cmd.Context(), e.Coin)
				if err != nil {
					return err
				}

				priceStr, err := wire.PriceToWire(price, info.SzDecimals, info.IsSpot)
				if err != nil {
					return err
				}
				sizeStr, err := wire.SizeToWire(size, info.SzDecimals)
				if err != nil {
					return err
				}

				params = append(params, exchange.OrderParams{
					AssetID:    info.AssetID,
					IsBuy:      side == "buy",
					Price:      priceStr,
					Size:       sizeStr,
					ReduceOnly: e.ReduceOnly,
					Tif:        wireTif,
					Cloid:      e.Cloid,
				})
			}

			action := exchange.BuildOrderAction(params, nil, builder)

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(action), nil)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.PlaceBatchOrders(cmd.Context(), action, expiresAfter)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("file", "", "path to JSON batch file")
	cmd.Flags().String("builder", "", "builder address for optional builder fee routing")
	cmd.Flags().Int("builder-fee-tenths-bp", 0, "builder fee in tenths of a basis point (requires --builder)")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	//nolint:errcheck // MarkFlagRequired on known flags never fails
	cmd.MarkFlagRequired("file")

	return cmd
}

// buildBatchResolver creates a resolver for batch entry resolution.
func buildBatchResolver(cfg *config.Config) resolver.Resolver {
	c := buildHTTPClient(cfg)
	cacheDir := resolveCacheDir(cfg)
	return resolver.NewResolver(c, cacheDir, time.Duration(cfg.MetadataTTL)*time.Second)
}
