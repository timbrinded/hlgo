package exchange

import (
	"context"
	"errors"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/wire"
)

// PlaceOrderInput bundles the raw user-provided parameters for placing an order.
type PlaceOrderInput struct {
	Coin       string
	Side       string // "buy" or "sell"
	Price      decimal.Decimal
	Size       decimal.Decimal
	Tif        string // "Gtc", "Ioc", "Alo"
	ReduceOnly bool
	Cloid      *string
	TpTrigger  *string // take-profit trigger price
	SlTrigger  *string // stop-loss trigger price
	Builder    *BuilderInfo
	// ExpiresAfter, when set, causes the action to be rejected after this Unix ms timestamp.
	ExpiresAfter *int64
	DryRun       bool
}

// PlaceMarketOrderInput bundles parameters for placing a market order.
// Market orders are implemented as aggressive IOC limit orders.
type PlaceMarketOrderInput struct {
	Coin            string
	Side            string // "buy" or "sell"
	Size            decimal.Decimal
	SlippagePercent decimal.Decimal
	Builder         *BuilderInfo
	// ExpiresAfter, when set, causes the action to be rejected after this Unix ms timestamp.
	ExpiresAfter *int64
	DryRun       bool
}

// PlaceMarketOrder executes an IOC convenience order at a slippage-adjusted price.
func (e *Executor) PlaceMarketOrder(ctx context.Context, input PlaceMarketOrderInput) (*PlaceOrderResult, error) {
	side := strings.ToLower(strings.TrimSpace(input.Side))
	if side != "buy" && side != "sell" {
		return nil, output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
			WithDetails("value", input.Side)
	}

	if input.SlippagePercent.IsNegative() {
		return nil, output.NewCLIError(output.ErrValidation, "slippage must be non-negative").
			WithDetails("value", input.SlippagePercent.String())
	}
	if input.SlippagePercent.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return nil, output.NewCLIError(output.ErrValidation, "slippage must be less than 100 percent").
			WithDetails("value", input.SlippagePercent.String())
	}

	assetInfo, err := e.resolver.ResolveAsset(ctx, input.Coin)
	if err != nil {
		return nil, err
	}

	canonicalCoin := assetInfo.CanonicalCoin
	if canonicalCoin == "" {
		canonicalCoin = assetInfo.Coin
	}
	if canonicalCoin == "" {
		canonicalCoin = input.Coin
	}

	mid, err := e.fetchMarketMid(ctx, input.Coin, canonicalCoin)
	if err != nil {
		return nil, err
	}

	slippage := input.SlippagePercent.Div(decimal.NewFromInt(100))
	price := mid.Mul(decimal.NewFromInt(1).Sub(slippage))
	if side == "buy" {
		price = mid.Mul(decimal.NewFromInt(1).Add(slippage))
	}
	price = wire.NearestValidPrice(price, assetInfo.SzDecimals, assetInfo.IsSpot)

	return e.PlaceOrder(ctx, PlaceOrderInput{
		Coin:         input.Coin,
		Side:         side,
		Price:        price,
		Size:         input.Size,
		Tif:          "Ioc",
		Builder:      input.Builder,
		ExpiresAfter: input.ExpiresAfter,
		DryRun:       input.DryRun,
	})
}

func marketCoinDex(coin string) string {
	idx := strings.Index(coin, ":")
	if idx <= 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(coin[:idx]))
}

func (e *Executor) fetchMarketMid(ctx context.Context, inputCoin, canonicalCoin string) (decimal.Decimal, error) {
	midsReq := map[string]string{"type": "allMids"}
	if dex := marketCoinDex(canonicalCoin); dex != "" {
		midsReq["dex"] = dex
	}

	midsRaw, err := e.client.PostInfo(ctx, midsReq)
	if err != nil {
		return decimal.Zero, err
	}
	mids, err := info.ParseMidsResult(midsRaw)
	if err != nil {
		return decimal.Zero, output.NewCLIError(output.ErrAPI, "failed to parse mids response").
			WithDetails("cause", err.Error())
	}

	midStr, ok := mids[canonicalCoin]
	if !ok {
		return decimal.Zero, output.NewCLIError(output.ErrValidation, "no mid price found for coin: "+canonicalCoin).
			WithDetails("coin", inputCoin).
			WithDetails("canonical_coin", canonicalCoin)
	}

	mid, err := decimal.NewFromString(midStr)
	if err != nil {
		return decimal.Zero, output.NewCLIError(output.ErrAPI, "invalid mid price from API").
			WithDetails("coin", canonicalCoin).
			WithDetails("value", midStr)
	}
	return mid, nil
}

func wrapTriggerPriceError(flag string, err error) error {
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		wrapped := output.NewCLIError(cliErr.Code, flag+" trigger price: "+cliErr.Message).
			WithDetails("flag", flag)
		for k, v := range cliErr.Details {
			wrapped = wrapped.WithDetails(k, v)
		}
		return wrapped
	}

	return output.NewCLIError(output.ErrValidation, flag+" trigger price validation failed").
		WithDetails("flag", flag).
		WithDetails("cause", err.Error())
}

func appendTriggerOrders(action *OrderAction, input PlaceOrderInput, assetID, szDecimals int, isSpot, isBuy bool, priceStr, sizeStr string) error {
	type triggerDef struct {
		flag         string
		value        *string
		parseMessage string
		tpsl         string
	}
	for _, trigger := range []triggerDef{{flag: "--tp", value: input.TpTrigger, parseMessage: "invalid take-profit trigger price", tpsl: "tp"}, {flag: "--sl", value: input.SlTrigger, parseMessage: "invalid stop-loss trigger price", tpsl: "sl"}} {
		if trigger.value == nil {
			continue
		}
		triggerPrice, err := decimal.NewFromString(*trigger.value)
		if err != nil {
			return output.NewCLIError(output.ErrValidation, trigger.parseMessage).
				WithDetails("value", *trigger.value)
		}
		triggerPx, err := wire.PriceToWire(triggerPrice, szDecimals, isSpot)
		if err != nil {
			return wrapTriggerPriceError(trigger.flag, err)
		}
		action.Orders = append(action.Orders, BuildOrderAction([]OrderParams{{AssetID: assetID, IsBuy: !isBuy, Price: priceStr, Size: sizeStr, ReduceOnly: true}}, []*TriggerParams{{TriggerPx: triggerPx, Tpsl: trigger.tpsl}}, nil).Orders...)
		action.Grouping = "normalTpsl"
	}
	return nil
}

// PlaceOrder executes the full order placement pipeline.
func (e *Executor) PlaceOrder(ctx context.Context, input PlaceOrderInput) (*PlaceOrderResult, error) {
	info, err := e.resolver.ResolveAsset(ctx, input.Coin)
	if err != nil {
		return nil, err
	}

	priceStr, err := wire.PriceToWire(input.Price, info.SzDecimals, info.IsSpot)
	if err != nil {
		return nil, err
	}

	sizeStr, err := wire.SizeToWire(input.Size, info.SzDecimals)
	if err != nil {
		return nil, err
	}

	isBuy := input.Side == "buy"

	action := BuildOrderAction([]OrderParams{{
		AssetID:    info.AssetID,
		IsBuy:      isBuy,
		Price:      priceStr,
		Size:       sizeStr,
		ReduceOnly: input.ReduceOnly,
		Tif:        input.Tif,
		Cloid:      input.Cloid,
	}}, nil, input.Builder)

	if err := appendTriggerOrders(action, input, info.AssetID, info.SzDecimals, info.IsSpot, isBuy, priceStr, sizeStr); err != nil {
		return nil, err
	}

	resolved := newResolvedOrder(info.Coin, info.AssetID, input.Side, priceStr, sizeStr, input.Tif, input.ReduceOnly, info.IsSpot)
	if input.DryRun {
		return &PlaceOrderResult{Action: action, Resolved: resolved}, nil
	}

	resp, err := e.executeL1Action(ctx, action, input.ExpiresAfter)
	if err != nil {
		return nil, err
	}
	return &PlaceOrderResult{Response: resp, Action: action, Resolved: resolved}, nil
}
