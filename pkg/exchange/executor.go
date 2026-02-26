package exchange

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
	"github.com/timbrinded/hlgo/pkg/wire"
)

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
	VaultAddr  string
	DryRun     bool
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

	// 4. Build triggers if present.
	var triggers []*TriggerParams
	if input.TpTrigger != nil {
		triggers = append(triggers, &TriggerParams{
			TriggerPx: *input.TpTrigger,
			Tpsl:      "tp",
		})
	}
	if input.SlTrigger != nil {
		triggers = append(triggers, &TriggerParams{
			TriggerPx: *input.SlTrigger,
			Tpsl:      "sl",
		})
	}

	// 5. Build order action.
	params := []OrderParams{{
		AssetID:    info.AssetID,
		IsBuy:      isBuy,
		Price:      priceStr,
		Size:       sizeStr,
		ReduceOnly: input.ReduceOnly,
		Tif:        input.Tif,
		Cloid:      input.Cloid,
	}}

	// For triggers: the main order uses the Limit tif, triggers are separate wires.
	// If no triggers, pass nil.
	var trigParams []*TriggerParams
	if len(triggers) == 0 {
		trigParams = nil
	} else {
		trigParams = make([]*TriggerParams, len(params))
		// Only attach triggers as additional orders, not to the main order.
		trigParams[0] = nil
	}
	action := BuildOrderAction(params, trigParams)

	// If triggers exist, add them as additional order wires.
	for _, trig := range triggers {
		trigOrder := OrderParams{
			AssetID:    info.AssetID,
			IsBuy:      !isBuy, // triggers are on the opposite side
			Price:      priceStr,
			Size:       sizeStr,
			ReduceOnly: true,
		}
		trigAction := BuildOrderAction([]OrderParams{trigOrder}, []*TriggerParams{trig})
		action.Orders = append(action.Orders, trigAction.Orders...)
		action.Grouping = "normalTpSl"
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

	sig, err := e.signer.SignL1Action(action, nonce, vaultAddr, e.mainnet)
	if err != nil {
		return nil, err
	}

	// 8. Send to exchange.
	resp, err := e.client.PostExchange(ctx, action, nonce, sig.Hex(), input.VaultAddr)
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
func (e *Executor) CancelOrders(ctx context.Context, cancels []CancelWire, vaultAddr string, dryRun bool) (json.RawMessage, error) {
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

	sig, err := e.signer.SignL1Action(action, nonce, vault, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, action, nonce, sig.Hex(), vaultAddr)
}

// CancelByCloid cancels orders by client order ID.
func (e *Executor) CancelByCloid(ctx context.Context, cancels []CancelByCloidWire, vaultAddr string, dryRun bool) (json.RawMessage, error) {
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

	sig, err := e.signer.SignL1Action(action, nonce, vault, e.mainnet)
	if err != nil {
		return nil, output.NewCLIError(output.ErrSigning, "failed to sign cancel-by-cloid action").
			WithDetails("cause", err.Error())
	}

	return e.client.PostExchange(ctx, action, nonce, sig.Hex(), vaultAddr)
}
