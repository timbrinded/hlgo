package exchange

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
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

// PlaceOrderResult holds the result of a place order operation.
type PlaceOrderResult struct {
	Response json.RawMessage `json:"response,omitempty"`
	Action   *OrderAction    `json:"action,omitempty"`
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
