package cmd

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
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
			filePath, _ := cmd.Flags().GetString("file") //nolint:errcheck // known flag

			params, err := loadBatchOrderParams(cmd, cfg, filePath)
			if err != nil {
				return err
			}
			builder, err := parseOptionalBuilder(cmd)
			if err != nil {
				return err
			}
			expiresAfter, err := parseOptionalExpiresAfter(cmd)
			if err != nil {
				return err
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

	mustMarkRequiredFlags(cmd, "file")

	return cmd
}

func loadBatchOrderParams(cmd *cobra.Command, cfg *config.Config, filePath string) ([]exchange.OrderParams, error) {
	data, err := os.ReadFile(filePath)
	var entries []batchEntry
	if err != nil {
		return nil, output.NewCLIError(output.ErrValidation, "failed to read batch file").
			WithDetails("path", filePath).
			WithDetails("cause", err.Error())
	} else if err := json.Unmarshal(data, &entries); err != nil {
		return nil, output.NewCLIError(output.ErrValidation, "invalid batch file JSON").
			WithDetails("path", filePath).
			WithDetails("cause", err.Error())
	} else if len(entries) == 0 {
		return nil, output.NewCLIError(output.ErrValidation, "batch file contains no orders").
			WithDetails("path", filePath)
	}

	r := buildResolver(cfg)
	params := make([]exchange.OrderParams, 0, len(entries))
	for i, entry := range entries {
		side := strings.ToLower(strings.TrimSpace(entry.Side))
		if side != "buy" && side != "sell" {
			return nil, batchEntryValidationError("invalid side in batch entry", i, entry.Side)
		}
		if entry.Tif == "" {
			entry.Tif = "gtc"
		}
		wireTIF, ok := resolveOrderTIF(entry.Tif)
		if !ok {
			return nil, batchEntryValidationError("invalid tif in batch entry", i, entry.Tif)
		}

		param := exchange.OrderParams{IsBuy: side == "buy", ReduceOnly: entry.ReduceOnly, Tif: wireTIF, Cloid: entry.Cloid}
		price, err := decimal.NewFromString(entry.Price)
		if err != nil {
			return nil, batchEntryValidationError("invalid price in batch entry", i, entry.Price)
		}
		size, err := decimal.NewFromString(entry.Size)
		if err != nil {
			return nil, batchEntryValidationError("invalid size in batch entry", i, entry.Size)
		}

		assetInfo, err := r.ResolveAsset(cmd.Context(), entry.Coin)
		if err != nil {
			return nil, err
		}
		param.AssetID = assetInfo.AssetID
		param.Price, err = wire.PriceToWire(price, assetInfo.SzDecimals, assetInfo.IsSpot)
		if err != nil {
			return nil, err
		}
		param.Size, err = wire.SizeToWire(size, assetInfo.SzDecimals)
		if err != nil {
			return nil, err
		}
		params = append(params, param)
	}
	return params, nil
}

func batchEntryValidationError(message string, index int, value string) error {
	return output.NewCLIError(output.ErrValidation, message).
		WithDetails("index", index).
		WithDetails("value", value)
}
