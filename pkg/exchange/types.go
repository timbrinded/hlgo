// Package exchange provides action types and builders for the Hyperliquid Exchange API.
package exchange

// LimitTif specifies the time-in-force for a limit order.
type LimitTif struct {
	Tif string `msgpack:"tif" json:"tif"`
}

// TriggerWire specifies trigger (TP/SL) parameters in wire format.
type TriggerWire struct {
	IsMarket  bool   `msgpack:"isMarket" json:"isMarket"`
	TriggerPx string `msgpack:"triggerPx" json:"triggerPx"`
	Tpsl      string `msgpack:"tpsl" json:"tpsl"`
}

// OrderTypeWire is the union of limit and trigger order types.
// Exactly one field should be non-nil.
type OrderTypeWire struct {
	Limit   *LimitTif    `msgpack:"limit,omitempty" json:"limit,omitempty"`
	Trigger *TriggerWire `msgpack:"trigger,omitempty" json:"trigger,omitempty"`
}

// OrderWire is the wire-format representation of a single order.
// Field names are abbreviated to match the Hyperliquid protocol.
type OrderWire struct {
	A int           `msgpack:"a" json:"a"`                     // asset ID
	B bool          `msgpack:"b" json:"b"`                     // is buy
	P string        `msgpack:"p" json:"p"`                     // price
	S string        `msgpack:"s" json:"s"`                     // size
	R bool          `msgpack:"r" json:"r"`                     // reduce only
	T OrderTypeWire `msgpack:"t" json:"t"`                     // order type
	C *string       `msgpack:"c,omitempty" json:"c,omitempty"` // client order ID
}

// OrderAction is the wire-format action for placing orders.
type OrderAction struct {
	Type     string       `msgpack:"type" json:"type"`
	Orders   []OrderWire  `msgpack:"orders" json:"orders"`
	Grouping string       `msgpack:"grouping" json:"grouping"`
	Builder  *BuilderInfo `msgpack:"builder,omitempty" json:"builder,omitempty"`
}

// BuilderInfo configures optional builder fee routing on order actions.
// b = builder address, f = fee in tenths of a basis point.
type BuilderInfo struct {
	B string `msgpack:"b" json:"b"`
	F int    `msgpack:"f" json:"f"`
}

// CancelWire is the wire-format for cancelling by OID.
type CancelWire struct {
	A int    `msgpack:"a" json:"a"` // asset ID
	O uint64 `msgpack:"o" json:"o"` // order ID
}

// CancelAction is the wire-format action for cancelling orders by OID.
type CancelAction struct {
	Type    string       `msgpack:"type" json:"type"`
	Cancels []CancelWire `msgpack:"cancels" json:"cancels"`
}

// CancelByCloidWire is the wire-format for cancelling by client order ID.
type CancelByCloidWire struct {
	Asset int    `msgpack:"asset" json:"asset"`
	Cloid string `msgpack:"cloid" json:"cloid"`
}

// CancelByCloidAction is the wire-format action for cancelling by client order ID.
type CancelByCloidAction struct {
	Type    string              `msgpack:"type" json:"type"`
	Cancels []CancelByCloidWire `msgpack:"cancels" json:"cancels"`
}

// UpdateLeverageAction is the wire-format action for setting leverage and margin mode.
type UpdateLeverageAction struct {
	Type     string `msgpack:"type" json:"type"`
	Asset    int    `msgpack:"asset" json:"asset"`
	IsCross  bool   `msgpack:"isCross" json:"isCross"`
	Leverage int    `msgpack:"leverage" json:"leverage"`
}

// UpdateIsolatedMarginAction is the wire-format action for adjusting isolated margin.
type UpdateIsolatedMarginAction struct {
	Type  string `msgpack:"type" json:"type"`
	Asset int    `msgpack:"asset" json:"asset"`
	IsBuy bool   `msgpack:"isBuy" json:"isBuy"`
	Ntli  int64  `msgpack:"ntli" json:"ntli"`
}

// ModifyAction is the wire-format action for modifying a single order.
type ModifyAction struct {
	Type  string    `msgpack:"type" json:"type"`
	Oid   uint64    `msgpack:"oid" json:"oid"`
	Order OrderWire `msgpack:"order" json:"order"`
}

// ScheduleCancelAction is the wire-format action for the dead man's switch.
type ScheduleCancelAction struct {
	Type string `msgpack:"type" json:"type"`
	Time *int64 `msgpack:"time,omitempty" json:"time,omitempty"`
}

// USDClassTransferAction transfers USDC between spot and perp classes.
type USDClassTransferAction struct {
	Type   string `msgpack:"type" json:"type"`
	Amount string `msgpack:"amount" json:"amount"`
	ToPerp bool   `msgpack:"toPerp" json:"toPerp"`
	Nonce  int64  `msgpack:"nonce" json:"nonce"`
}

// Withdraw3Action withdraws USDC from Hyperliquid to an Arbitrum address.
type Withdraw3Action struct {
	Type        string `msgpack:"type" json:"type"`
	Destination string `msgpack:"destination" json:"destination"`
	Amount      string `msgpack:"amount" json:"amount"`
	Time        int64  `msgpack:"time" json:"time"`
}

// ClassTransferAction performs class transfer using the classTransfer action type.
type ClassTransferAction struct {
	Type   string `msgpack:"type" json:"type"`
	Amount string `msgpack:"amount" json:"amount"`
	ToPerp bool   `msgpack:"toPerp" json:"toPerp"`
	Nonce  int64  `msgpack:"nonce" json:"nonce"`
}

// SpotSendAction sends a spot token to another address.
type SpotSendAction struct {
	Type        string `msgpack:"type" json:"type"`
	Destination string `msgpack:"destination" json:"destination"`
	Token       string `msgpack:"token" json:"token"`
	Amount      string `msgpack:"amount" json:"amount"`
	Time        int64  `msgpack:"time" json:"time"`
}

// ApproveAgentAction approves an agent wallet for trading on behalf of the master wallet.
type ApproveAgentAction struct {
	Type         string `msgpack:"type" json:"type"`
	AgentAddress string `msgpack:"agentAddress" json:"agentAddress"`
	AgentName    string `msgpack:"agentName,omitempty" json:"agentName,omitempty"`
	Nonce        int64  `msgpack:"nonce" json:"nonce"`
}

// UserSetAbstractionAction sets account abstraction behavior for a user address.
type UserSetAbstractionAction struct {
	Type        string `msgpack:"type" json:"type"`
	User        string `msgpack:"user" json:"user"`
	Abstraction string `msgpack:"abstraction" json:"abstraction"`
	Nonce       int64  `msgpack:"nonce" json:"nonce"`
}
