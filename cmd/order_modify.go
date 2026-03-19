package cmd

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newOrderModifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Modify an existing order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")               //nolint:errcheck // known flag
			oidStr, _ := cmd.Flags().GetString("oid")              //nolint:errcheck // known flag
			sideFlag, _ := cmd.Flags().GetString("side")           //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price")          //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")            //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")             //nolint:errcheck // known flag
			reduce, _ := cmd.Flags().GetBool("reduce")             //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of") //nolint:errcheck // known flag

			input, err := parseModifyBaseInput(coin, oidStr, sideFlag, tifFlag, reduce, cfg.DryRun)
			if err != nil {
				return err
			}
			priceStr, sizeStr, err = backfillModifyPriceSize(cmd, cfg, input.Oid, coin, onBehalfOf, priceStr, sizeStr)
			if err != nil {
				return err
			}
			if input.Price, err = parseDecimalField("price", priceStr); err != nil {
				return err
			}
			if input.Size, err = parseDecimalField("size", sizeStr); err != nil {
				return err
			}
			if input.ExpiresAfter, err = parseOptionalExpiresAfter(cmd); err != nil {
				return err
			}
			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}
			result, err := exec.ModifyOrder(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("oid", "", "order ID to modify")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("price", "", "new limit price (optional if --size is set)")
	cmd.Flags().String("size", "", "new order size (optional if --price is set)")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().Bool("reduce", false, "reduce-only order")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	mustMarkRequiredFlags(cmd, "coin", "oid", "side")

	return cmd
}

func parseModifyBaseInput(coin, oidStr, sideFlag, tifFlag string, reduce, dryRun bool) (exchange.ModifyOrderInput, error) {
	var err error
	input := exchange.ModifyOrderInput{Coin: coin, ReduceOnly: reduce, DryRun: dryRun}
	input.Side, err = parseOrderSide(sideFlag)
	if err != nil {
		return exchange.ModifyOrderInput{}, err
	}
	input.Oid, err = strconv.ParseUint(oidStr, 10, 64)
	if err != nil {
		return exchange.ModifyOrderInput{}, output.NewCLIError(output.ErrValidation, "invalid OID: must be numeric").
			WithDetails("value", oidStr)
	}
	input.Tif, err = parseOrderTIF(tifFlag)
	if err != nil {
		return exchange.ModifyOrderInput{}, err
	}
	return input, nil
}

func backfillModifyPriceSize(cmd *cobra.Command, cfg *config.Config, oid uint64, coin, onBehalfOf, priceStr, sizeStr string) (string, string, error) {
	hasPrice, hasSize := cmd.Flags().Changed("price"), cmd.Flags().Changed("size")
	if !hasPrice && !hasSize {
		return "", "", output.NewCLIError(output.ErrValidation, "at least one of --price or --size is required")
	}
	if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
		return "", "", output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
			WithDetails("on_behalf_of", onBehalfOf)
	}
	if !hasPrice || !hasSize {
		existing, err := lookupOpenOrderByOID(cmd, cfg, oid, coin, onBehalfOf)
		if err != nil {
			return "", "", err
		}
		if !hasPrice {
			priceStr = existing.LimitPx
		}
		if !hasSize {
			sizeStr = existing.Sz
		}
	}
	return priceStr, sizeStr, nil
}

func lookupOpenOrderByOID(cmd *cobra.Command, cfg *config.Config, oid uint64, coin string, onBehalfOf string) (*info.OpenOrder, error) {
	addr, err := info.ResolveUserAddress(onBehalfOf, cfg)
	if err != nil {
		return nil, err
	}

	dex := cfg.Dex
	if dex == "" {
		if idx := strings.Index(coin, ":"); idx > 0 {
			dex = strings.ToLower(strings.TrimSpace(coin[:idx]))
		}
	}
	_, orders, err := fetchOpenOrders(cmd, cfg, addr, dex)
	if err != nil {
		return nil, err
	}

	for _, ord := range orders {
		if ord.Oid < 0 {
			continue
		}
		if uint64(ord.Oid) != oid {
			continue
		}
		if coin != "" && !strings.EqualFold(ord.Coin, coin) {
			continue
		}
		order := ord
		return &order, nil
	}

	return nil, output.NewCLIError(output.ErrValidation, "order not found for modify backfill").
		WithDetails("oid", oid).
		WithDetails("coin", coin).
		WithDetails("hint", "ensure the order is still open or provide both --price and --size")
}
