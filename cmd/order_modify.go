package cmd

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
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
			coin, _ := cmd.Flags().GetString("coin")                     //nolint:errcheck // known flag
			oidStr, _ := cmd.Flags().GetString("oid")                    //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")                     //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price")                //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")                  //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")                   //nolint:errcheck // known flag
			reduce, _ := cmd.Flags().GetBool("reduce")                   //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of")       //nolint:errcheck // known flag
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

			hasPrice := cmd.Flags().Changed("price")
			hasSize := cmd.Flags().Changed("size")
			if !hasPrice && !hasSize {
				return output.NewCLIError(output.ErrValidation, "at least one of --price or --size is required")
			}

			if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
				return output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
					WithDetails("on_behalf_of", onBehalfOf)
			}

			if !hasPrice || !hasSize {
				existing, err := lookupOpenOrderByOID(cmd, cfg, oid, coin, onBehalfOf)
				if err != nil {
					return err
				}
				if !hasPrice {
					priceStr = existing.LimitPx
				}
				if !hasSize {
					sizeStr = existing.Sz
				}
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
				OnBehalfOf:   onBehalfOf,
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
	cmd.Flags().String("price", "", "new limit price (optional if --size is set)")
	cmd.Flags().String("size", "", "new order size (optional if --price is set)")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().Bool("reduce", false, "reduce-only order")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	for _, required := range []string{"coin", "oid", "side"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}

func lookupOpenOrderByOID(cmd *cobra.Command, cfg *config.Config, oid uint64, coin string, onBehalfOf string) (*info.OpenOrder, error) {
	addr := onBehalfOf
	if addr == "" {
		var err error
		addr, err = info.ResolveUserAddress("", cfg)
		if err != nil {
			return nil, err
		}
	}

	ic := buildInfoClient(cfg)
	dex := cfg.Dex
	if dex == "" {
		if idx := strings.Index(coin, ":"); idx > 0 {
			dex = strings.ToLower(strings.TrimSpace(coin[:idx]))
		}
	}
	raw, err := ic.FrontendOpenOrders(cmd.Context(), addr, dex)
	if err != nil {
		return nil, err
	}

	orders, err := info.ParseOpenOrdersResult(raw)
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
