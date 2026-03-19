package exchange

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/wire"
)

// UpdateLeverageInput holds parameters for updating leverage.
type UpdateLeverageInput struct {
	Coin     string
	IsCross  bool
	Leverage int
	DryRun   bool
}

// UpdateIsolatedMarginInput holds parameters for adjusting isolated margin.
type UpdateIsolatedMarginInput struct {
	Coin   string
	IsBuy  bool
	Amount decimal.Decimal
	DryRun bool
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
	DryRun       bool
}

// ScheduleCancelInput holds parameters for the dead man's switch.
type ScheduleCancelInput struct {
	Time   *int64
	DryRun bool
}

// CancelOrders cancels orders by OID.
func (e *Executor) CancelOrders(ctx context.Context, cancels []CancelWire, dryRun bool, expiresAfter *int64) (json.RawMessage, error) {
	action := BuildCancelAction(cancels)

	if dryRun {
		return json.Marshal(action)
	}

	return e.executeL1Action(ctx, action, expiresAfter)
}

// CancelByCloid cancels orders by client order ID.
func (e *Executor) CancelByCloid(ctx context.Context, cancels []CancelByCloidWire, dryRun bool, expiresAfter *int64) (json.RawMessage, error) {
	action := BuildCancelByCloidAction(cancels)

	if dryRun {
		return json.Marshal(action)
	}

	return e.executeL1Action(ctx, action, expiresAfter)
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

	return e.executeL1Action(ctx, action, nil)
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
	ntliInt, err := strconv.ParseInt(ntli.String(), 10, 64)
	if err != nil {
		return nil, output.NewCLIError(output.ErrValidation, "amount is out of range").
			WithDetails("value", input.Amount.String())
	}

	action := BuildUpdateIsolatedMarginAction(info.AssetID, input.IsBuy, ntliInt)

	if input.DryRun {
		return json.Marshal(action)
	}

	return e.executeL1Action(ctx, action, nil)
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

	resolved := newResolvedOrder(info.Coin, info.AssetID, input.Side, priceStr, sizeStr, input.Tif, input.ReduceOnly, info.IsSpot)
	if input.DryRun {
		return &ModifyOrderResult{Action: action, Resolved: resolved}, nil
	}

	resp, err := e.executeL1Action(ctx, action, input.ExpiresAfter)
	if err != nil {
		return nil, err
	}
	return &ModifyOrderResult{Response: resp, Resolved: resolved}, nil
}

// executeL1Action signs and posts a standard L1 action.
// On-behalf trading context is handled by agent authorization, not vaultAddress.
func (e *Executor) executeL1Action(ctx context.Context, action any, expiresAfter *int64) (json.RawMessage, error) {
	nonce := time.Now().UnixMilli()
	sig, err := e.signer.SignL1Action(action, nonce, nil, expiresAfter, e.mainnet)
	if err != nil {
		return nil, err
	}
	return e.client.PostExchange(ctx, client.ExchangeRequest{Action: action, Nonce: nonce, Signature: sigToWire(sig), ExpiresAfter: expiresAfter})
}

// PlaceBatchOrders signs and sends a pre-built OrderAction for batch order placement.
func (e *Executor) PlaceBatchOrders(ctx context.Context, action *OrderAction, expiresAfter *int64) (json.RawMessage, error) {
	return e.executeL1Action(ctx, action, expiresAfter)
}

// ScheduleCancel sets or clears the dead man's switch for order cancellation.
func (e *Executor) ScheduleCancel(ctx context.Context, input ScheduleCancelInput) (json.RawMessage, error) {
	action := BuildScheduleCancelAction(input.Time)

	if input.DryRun {
		return json.Marshal(action)
	}

	return e.executeL1Action(ctx, action, nil)
}
