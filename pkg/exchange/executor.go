package exchange

import (
	"encoding/hex"
	"encoding/json"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
)

// sigToWire converts a signer.Signature to the structured wire format expected by the exchange API.
func sigToWire(sig *signer.Signature) client.SignatureWire {
	return client.SignatureWire{R: "0x" + hex.EncodeToString(sig.R[:]), S: "0x" + hex.EncodeToString(sig.S[:]), V: int(sig.V)}
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
	return &Executor{signer: s, client: c, resolver: r, mainnet: mainnet}
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

func newResolvedOrder(coin string, assetID int, side, price, size, tif string, reduceOnly, isSpot bool) *ResolvedOrder {
	return &ResolvedOrder{Coin: coin, AssetID: assetID, Side: side, Price: price, Size: size, Tif: tif, ReduceOnly: reduceOnly, IsSpot: isSpot}
}
