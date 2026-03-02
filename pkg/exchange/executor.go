package exchange

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
	"github.com/timbrinded/hlgo/pkg/wire"
)

// sigToWire converts a signer.Signature to the structured wire format expected by the exchange API.
func sigToWire(sig *signer.Signature) client.SignatureWire {
	return client.SignatureWire{
		R: "0x" + hex.EncodeToString(sig.R[:]),
		S: "0x" + hex.EncodeToString(sig.S[:]),
		V: int(sig.V),
	}
}

// Executor orchestrates the resolve → validate → sign → send pipeline for exchange actions.
type Executor struct {
	signer   signer.Signer
	client   *client.Client
	resolver resolver.Resolver
	mainnet  bool
}

// NewExecutor creates an Executor with the given dependencies.
func NewExecutor(s signer.Signer, c *client.Client, r resolver.Resolver, mainnet bool) *Executor {
	return &Executor{
		signer:   s,
		client:   c,
		resolver: r,
		mainnet:  mainnet,
	}
}

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
	VaultAddr    string
	DryRun       bool
}

// UpdateLeverageInput holds parameters for updating leverage.
type UpdateLeverageInput struct {
	Coin      string
	IsCross   bool
	Leverage  int
	VaultAddr string
	DryRun    bool
}

// UpdateIsolatedMarginInput holds parameters for adjusting isolated margin.
type UpdateIsolatedMarginInput struct {
	Coin      string
	IsBuy     bool
	Amount    decimal.Decimal
	VaultAddr string
	DryRun    bool
}

// ModifyOrderInput holds parameters for modifying an existing order.
type ModifyOrderInput struct {
	Coin         string
	Oid          uint64
	Side         string
	Price        decimal.Decimal
	Size         decimal.Decimal
	Tif          string // "Gtc", "Ioc", "Alo"
	ReduceOnly   bool
	Cloid        *string
	ExpiresAfter *int64
	VaultAddr    string
	DryRun       bool
}

// ScheduleCancelInput holds parameters for the dead man's switch.
type ScheduleCancelInput struct {
	Time   *int64
	DryRun bool
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
	VaultAddr    string
	DryRun       bool
}

// PlaceOrderResult holds the result of a place order operation.
type PlaceOrderResult struct {
	Response json.RawMessage `json:"response,omitempty"`
	Action   *OrderAction    `json:"action,omitempty"`
	Resolved *ResolvedOrder  `json:"resolved,omitempty"`
}

// ModifyOrderResult holds the result of a modify order operation.
type ModifyOrderResult struct {
	Response json.RawMessage `json:"response,omitempty"`
	Action   *ModifyAction   `json:"action,omitempty"`
	Resolved *ResolvedOrder  `json:"resolved,omitempty"`
}

// ResolvedOrder holds the resolved and validated order details for dry-run output.
type ResolvedOrder struct {
	Coin       string `json:"coin"`
	AssetID    int    `json:"asset_id"`
	Side       string `json:"side"`
	Price      string `json:"price"`
	Size       string `json:"size"`
	Tif        string `json:"tif"`
	ReduceOnly bool   `json:"reduce_only"`
	IsSpot     bool   `json:"is_spot"`
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

	midsReq := map[string]string{"type": "allMids"}
	if dex := marketCoinDex(canonicalCoin); dex != "" {
		midsReq["dex"] = dex
	}

	midsRaw, err := e.client.PostInfo(ctx, midsReq)
	if err != nil {
		return nil, err
	}

	mids, err := info.ParseMidsResult(midsRaw)
	if err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to parse mids response").
			WithDetails("cause", err.Error())
	}

	midStr, ok := mids[canonicalCoin]
	if !ok {
		return nil, output.NewCLIError(output.ErrValidation, "no mid price found for coin: "+canonicalCoin).
			WithDetails("coin", input.Coin).
			WithDetails("canonical_coin", canonicalCoin)
	}

	mid, err := decimal.NewFromString(midStr)
	if err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "invalid mid price from API").
			WithDetails("coin", canonicalCoin).
			WithDetails("value", midStr)
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
		VaultAddr:    input.VaultAddr,
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

// PlaceOrder executes the full order placement pipeline.
func (e *Executor) PlaceOrder(ctx context.Context, input PlaceOrderInput) (*PlaceOrderResult, error) {
	// 1. Resolve coin → asset info.
	info, err := e.resolver.ResolveAsset(ctx, input.Coin)
	if err != nil {
		return nil, err
	}

	// 2. Validate and format price.
	priceStr, err := wire.PriceToWire(input.Price, info.SzDecimals, info.IsSpot)
	if err != nil {
		return nil, err
	}

	// 3. Format size.
	sizeStr, err := wire.SizeToWire(input.Size, info.SzDecimals)
	if err != nil {
		return nil, err
	}

	isBuy := input.Side == "buy"

	// 4. Build the main limit order (no trigger params — those are separate wires).
	action := BuildOrderAction([]OrderParams{{
		AssetID:    info.AssetID,
		IsBuy:      isBuy,
		Price:      priceStr,
		Size:       sizeStr,
		ReduceOnly: input.ReduceOnly,
		Tif:        input.Tif,
		Cloid:      input.Cloid,
	}}, nil, input.Builder)

	// 5. Append TP/SL trigger wires if present.
	// Each trigger is a separate reduce-only order on the opposite side, using the same
	// size as the main order. The trigger price goes in TriggerPx (IsMarket=true means
	// the trigger fires a market order at that level), while Price/Size are wire-required
	// fields that match the main order.
	type triggerDef struct {
		px   string
		tpsl string
	}
	var triggers []triggerDef
	if input.TpTrigger != nil {
		triggers = append(triggers, triggerDef{px: *input.TpTrigger, tpsl: "tp"})
	}
	if input.SlTrigger != nil {
		triggers = append(triggers, triggerDef{px: *input.SlTrigger, tpsl: "sl"})
	}
	for _, trig := range triggers {
		trigOrder := OrderParams{
			AssetID:    info.AssetID,
			IsBuy:      !isBuy,
			Price:      priceStr,
			Size:       sizeStr,
			ReduceOnly: true,
		}
		trigAction := BuildOrderAction([]OrderParams{trigOrder}, []*TriggerParams{{
			TriggerPx: trig.px,
			Tpsl:      trig.tpsl,
		}}, nil)
		action.Orders = append(action.Orders, trigAction.Orders...)
		action.Grouping = "normalTpsl"
	}

	resolved := &ResolvedOrder{
		Coin:       info.Coin,
		AssetID:    info.AssetID,
		Side:       input.Side,
		Price:      priceStr,
		Size:       sizeStr,
		Tif:        input.Tif,
		ReduceOnly: input.ReduceOnly,
		IsSpot:     info.IsSpot,
	}

	// 6. Dry-run: return action without signing/sending.
	if input.DryRun {
		return &PlaceOrderResult{
			Action:   action,
			Resolved: resolved,
		}, nil
	}

	// 7. Generate nonce and sign.
	nonce := time.Now().UnixMilli()

	var vaultAddr *common.Address
	if input.VaultAddr != "" {
		a := common.HexToAddress(input.VaultAddr)
		vaultAddr = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vaultAddr, input.ExpiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}

	// 8. Send to exchange.
	resp, err := e.client.PostExchange(ctx, action, nonce, sigToWire(sig), input.VaultAddr, input.ExpiresAfter)
	if err != nil {
		return nil, err
	}

	return &PlaceOrderResult{
		Response: resp,
		Action:   action,
		Resolved: resolved,
	}, nil
}

// CancelOrders cancels orders by OID.
func (e *Executor) CancelOrders(ctx context.Context, cancels []CancelWire, vaultAddr string, dryRun bool, expiresAfter *int64) (json.RawMessage, error) {
	action := BuildCancelAction(cancels)

	if dryRun {
		return json.Marshal(action)
	}

	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if vaultAddr != "" {
		a := common.HexToAddress(vaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, expiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), vaultAddr, expiresAfter)
}

// CancelByCloid cancels orders by client order ID.
func (e *Executor) CancelByCloid(ctx context.Context, cancels []CancelByCloidWire, vaultAddr string, dryRun bool, expiresAfter *int64) (json.RawMessage, error) {
	action := BuildCancelByCloidAction(cancels)

	if dryRun {
		return json.Marshal(action)
	}

	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if vaultAddr != "" {
		a := common.HexToAddress(vaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, expiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), vaultAddr, expiresAfter)
}

// UpdateLeverage sets leverage and margin mode for a coin.
func (e *Executor) UpdateLeverage(ctx context.Context, input UpdateLeverageInput) (json.RawMessage, error) {
	if input.Leverage < 1 {
		return nil, output.NewCLIError(output.ErrValidation, "leverage must be at least 1").
			WithDetails("value", input.Leverage)
	}

	info, err := e.resolver.ResolveAsset(ctx, input.Coin)
	if err != nil {
		return nil, err
	}

	action := BuildUpdateLeverageAction(info.AssetID, input.IsCross, input.Leverage)

	if input.DryRun {
		return json.Marshal(action)
	}

	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if input.VaultAddr != "" {
		a := common.HexToAddress(input.VaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, nil, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), input.VaultAddr, nil)
}

// UpdateIsolatedMargin adjusts isolated margin for a position.
func (e *Executor) UpdateIsolatedMargin(ctx context.Context, input UpdateIsolatedMarginInput) (json.RawMessage, error) {
	info, err := e.resolver.ResolveAsset(ctx, input.Coin)
	if err != nil {
		return nil, err
	}

	// Convert decimal amount to integer ntli (micro-units: amount * 1_000_000).
	ntli := input.Amount.Mul(decimal.NewFromInt(1_000_000))
	if !ntli.IsInteger() {
		return nil, output.NewCLIError(output.ErrValidation, "amount precision exceeds 6 decimal places").
			WithDetails("value", input.Amount.String())
	}

	action := BuildUpdateIsolatedMarginAction(info.AssetID, input.IsBuy, ntli.IntPart())

	if input.DryRun {
		return json.Marshal(action)
	}

	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if input.VaultAddr != "" {
		a := common.HexToAddress(input.VaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, nil, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), input.VaultAddr, nil)
}

// ModifyOrder modifies an existing order.
func (e *Executor) ModifyOrder(ctx context.Context, input ModifyOrderInput) (*ModifyOrderResult, error) {
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

	orderWire := OrderWire{
		A: info.AssetID,
		B: isBuy,
		P: priceStr,
		S: sizeStr,
		R: input.ReduceOnly,
		T: OrderTypeWire{
			Limit: &LimitTif{Tif: input.Tif},
		},
		C: input.Cloid,
	}

	action := BuildModifyAction(input.Oid, orderWire)

	resolved := &ResolvedOrder{
		Coin:       info.Coin,
		AssetID:    info.AssetID,
		Side:       input.Side,
		Price:      priceStr,
		Size:       sizeStr,
		Tif:        input.Tif,
		ReduceOnly: input.ReduceOnly,
		IsSpot:     info.IsSpot,
	}

	if input.DryRun {
		return &ModifyOrderResult{
			Action:   action,
			Resolved: resolved,
		}, nil
	}

	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if input.VaultAddr != "" {
		a := common.HexToAddress(input.VaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, input.ExpiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.PostExchange(ctx, action, nonce, sigToWire(sig), input.VaultAddr, input.ExpiresAfter)
	if err != nil {
		return nil, err
	}

	return &ModifyOrderResult{
		Response: resp,
		Resolved: resolved,
	}, nil
}

// PlaceBatchOrders signs and sends a pre-built OrderAction for batch order placement.
func (e *Executor) PlaceBatchOrders(ctx context.Context, action *OrderAction, vaultAddr string, expiresAfter *int64) (json.RawMessage, error) {
	nonce := time.Now().UnixMilli()

	var vault *common.Address
	if vaultAddr != "" {
		a := common.HexToAddress(vaultAddr)
		vault = &a
	}

	sig, err := e.signer.SignL1Action(action, nonce, vault, expiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), vaultAddr, expiresAfter)
}

// ScheduleCancel sets or clears the dead man's switch for order cancellation.
func (e *Executor) ScheduleCancel(ctx context.Context, input ScheduleCancelInput) (json.RawMessage, error) {
	action := BuildScheduleCancelAction(input.Time)

	if input.DryRun {
		return json.Marshal(action)
	}

	nonce := time.Now().UnixMilli()

	// ScheduleCancel does not support vault addresses — the Hyperliquid API
	// applies the dead man's switch to the signing wallet only.
	sig, err := e.signer.SignL1Action(action, nonce, nil, nil, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sigToWire(sig), "", nil)
}
