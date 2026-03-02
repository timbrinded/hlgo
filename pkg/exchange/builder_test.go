package exchange

import (
	"encoding/json"
	"testing"
)

func TestBuildOrderAction_LimitOrder(t *testing.T) {
	params := []OrderParams{{
		AssetID:    0,
		IsBuy:      true,
		Price:      "50000",
		Size:       "0.01",
		ReduceOnly: false,
		Tif:        "Gtc",
	}}

	action := BuildOrderAction(params, nil, nil)

	if action.Type != "order" {
		t.Errorf("Type = %q, want order", action.Type)
	}
	if action.Grouping != "na" {
		t.Errorf("Grouping = %q, want na", action.Grouping)
	}
	if len(action.Orders) != 1 {
		t.Fatalf("Orders len = %d, want 1", len(action.Orders))
	}

	o := action.Orders[0]
	if o.A != 0 || !o.B || o.P != "50000" || o.S != "0.01" || o.R {
		t.Errorf("order wire = %+v", o)
	}
	if o.T.Limit == nil || o.T.Limit.Tif != "Gtc" {
		t.Errorf("expected Limit TIF Gtc, got %+v", o.T)
	}
}

func TestBuildOrderAction_WithTrigger(t *testing.T) {
	params := []OrderParams{{
		AssetID: 0,
		IsBuy:   true,
		Price:   "50000",
		Size:    "0.01",
		Tif:     "Gtc",
	}}
	triggers := []*TriggerParams{{
		TriggerPx: "48000",
		Tpsl:      "sl",
	}}

	action := BuildOrderAction(params, triggers, nil)

	if action.Grouping != "normalTpsl" {
		t.Errorf("Grouping = %q, want normalTpSl", action.Grouping)
	}
	if action.Orders[0].T.Trigger == nil {
		t.Fatal("expected trigger on order")
	}
	if action.Orders[0].T.Trigger.Tpsl != "sl" {
		t.Errorf("Tpsl = %q, want sl", action.Orders[0].T.Trigger.Tpsl)
	}
}

func TestBuildOrderAction_WithCloid(t *testing.T) {
	cloid := "my-order-123"
	params := []OrderParams{{
		AssetID: 0,
		IsBuy:   true,
		Price:   "50000",
		Size:    "0.01",
		Tif:     "Gtc",
		Cloid:   &cloid,
	}}

	action := BuildOrderAction(params, nil, nil)

	if action.Orders[0].C == nil || *action.Orders[0].C != "my-order-123" {
		t.Errorf("Cloid = %v, want my-order-123", action.Orders[0].C)
	}
}

func TestBuildCancelAction(t *testing.T) {
	cancels := []CancelWire{
		{A: 0, O: 12345},
		{A: 1, O: 67890},
	}

	action := BuildCancelAction(cancels)

	if action.Type != "cancel" {
		t.Errorf("Type = %q, want cancel", action.Type)
	}
	if len(action.Cancels) != 2 {
		t.Fatalf("Cancels len = %d, want 2", len(action.Cancels))
	}
	if action.Cancels[0].O != 12345 {
		t.Errorf("first cancel OID = %d, want 12345", action.Cancels[0].O)
	}
}

func TestBuildOrderAction_WithBuilder(t *testing.T) {
	params := []OrderParams{{
		AssetID: 0,
		IsBuy:   true,
		Price:   "50000",
		Size:    "0.01",
		Tif:     "Gtc",
	}}
	builder := &BuilderInfo{
		B: "0x1234567890abcdef1234567890abcdef12345678",
		F: 10,
	}

	action := BuildOrderAction(params, nil, builder)

	if action.Builder == nil {
		t.Fatal("Builder = nil, want non-nil")
	}
	if action.Builder.B != builder.B {
		t.Errorf("Builder.B = %q, want %q", action.Builder.B, builder.B)
	}
	if action.Builder.F != builder.F {
		t.Errorf("Builder.F = %d, want %d", action.Builder.F, builder.F)
	}
}

func TestBuildCancelByCloidAction(t *testing.T) {
	cancels := []CancelByCloidWire{
		{Asset: 0, Cloid: "abc"},
	}

	action := BuildCancelByCloidAction(cancels)

	if action.Type != "cancelByCloid" {
		t.Errorf("Type = %q, want cancelByCloid", action.Type)
	}
	if len(action.Cancels) != 1 || action.Cancels[0].Cloid != "abc" {
		t.Errorf("Cancels = %+v", action.Cancels)
	}
}

func TestBuildUpdateLeverageAction(t *testing.T) {
	action := BuildUpdateLeverageAction(1, true, 10)

	if action.Type != "updateLeverage" {
		t.Errorf("Type = %q, want updateLeverage", action.Type)
	}
	if action.Asset != 1 {
		t.Errorf("Asset = %d, want 1", action.Asset)
	}
	if !action.IsCross {
		t.Error("IsCross = false, want true")
	}
	if action.Leverage != 10 {
		t.Errorf("Leverage = %d, want 10", action.Leverage)
	}
}

func TestBuildUpdateIsolatedMarginAction(t *testing.T) {
	action := BuildUpdateIsolatedMarginAction(0, true, 100500000)

	if action.Type != "updateIsolatedMargin" {
		t.Errorf("Type = %q, want updateIsolatedMargin", action.Type)
	}
	if action.Asset != 0 {
		t.Errorf("Asset = %d, want 0", action.Asset)
	}
	if !action.IsBuy {
		t.Error("IsBuy = false, want true")
	}
	if action.Ntli != 100500000 {
		t.Errorf("Ntli = %d, want 100500000", action.Ntli)
	}
}

func TestBuildUpdateIsolatedMarginAction_NegativeNtli(t *testing.T) {
	action := BuildUpdateIsolatedMarginAction(0, false, -50000000)

	if action.Ntli != -50000000 {
		t.Errorf("Ntli = %d, want -50000000", action.Ntli)
	}
}

func TestBuildModifyAction(t *testing.T) {
	order := OrderWire{
		A: 0,
		B: true,
		P: "50000",
		S: "0.01",
		R: false,
		T: OrderTypeWire{
			Limit: &LimitTif{Tif: "Gtc"},
		},
	}

	action := BuildModifyAction(12345, order)

	if action.Type != "modify" {
		t.Errorf("Type = %q, want modify", action.Type)
	}
	if action.Oid != 12345 {
		t.Errorf("Oid = %d, want 12345", action.Oid)
	}
	if action.Order.A != 0 {
		t.Errorf("Order.A = %d, want 0", action.Order.A)
	}
	if !action.Order.B {
		t.Error("Order.B = false, want true")
	}
	if action.Order.P != "50000" {
		t.Errorf("Order.P = %q, want 50000", action.Order.P)
	}
	if action.Order.S != "0.01" {
		t.Errorf("Order.S = %q, want 0.01", action.Order.S)
	}
}

func TestBuildScheduleCancelAction(t *testing.T) {
	cancelTime := int64(1700000000000)
	action := BuildScheduleCancelAction(&cancelTime)

	if action.Type != "scheduleCancel" {
		t.Errorf("Type = %q, want scheduleCancel", action.Type)
	}
	if action.Time == nil {
		t.Fatal("Time = nil, want non-nil")
	}
	if *action.Time != 1700000000000 {
		t.Errorf("Time = %d, want 1700000000000", *action.Time)
	}
}

func TestBuildScheduleCancelAction_Clear(t *testing.T) {
	action := BuildScheduleCancelAction(nil)

	if action.Type != "scheduleCancel" {
		t.Errorf("Type = %q, want scheduleCancel", action.Type)
	}
	if action.Time != nil {
		t.Errorf("Time = %d, want nil", *action.Time)
	}
}

func TestBuildUSDClassTransferAction(t *testing.T) {
	action := BuildUSDClassTransferAction("250.5", true, 1700000000000)

	if action.Type != "usdClassTransfer" {
		t.Errorf("Type = %q, want usdClassTransfer", action.Type)
	}
	if action.Amount != "250.5" {
		t.Errorf("Amount = %q, want 250.5", action.Amount)
	}
	if !action.ToPerp {
		t.Error("ToPerp = false, want true")
	}
	if action.Nonce != 1700000000000 {
		t.Errorf("Nonce = %d, want 1700000000000", action.Nonce)
	}
}

func TestBuildWithdraw3Action(t *testing.T) {
	action := BuildWithdraw3Action("0x1234567890abcdef1234567890abcdef12345678", "12.34", 1700000000001)

	if action.Type != "withdraw3" {
		t.Errorf("Type = %q, want withdraw3", action.Type)
	}
	if action.Destination != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("Destination = %q", action.Destination)
	}
	if action.Amount != "12.34" {
		t.Errorf("Amount = %q, want 12.34", action.Amount)
	}
	if action.Time != 1700000000001 {
		t.Errorf("Time = %d, want 1700000000001", action.Time)
	}
}

func TestBuildClassTransferAction(t *testing.T) {
	action := BuildClassTransferAction("1.5", false, 1700000000002)

	if action.Type != "classTransfer" {
		t.Errorf("Type = %q, want classTransfer", action.Type)
	}
	if action.Amount != "1.5" {
		t.Errorf("Amount = %q, want 1.5", action.Amount)
	}
	if action.ToPerp {
		t.Error("ToPerp = true, want false")
	}
	if action.Nonce != 1700000000002 {
		t.Errorf("Nonce = %d, want 1700000000002", action.Nonce)
	}
}

func TestBuildSpotSendAction(t *testing.T) {
	action := BuildSpotSendAction("0xabc", "PURR:0x1", "3.14", 1700000000003)

	if action.Type != "spotSend" {
		t.Errorf("Type = %q, want spotSend", action.Type)
	}
	if action.Destination != "0xabc" {
		t.Errorf("Destination = %q, want 0xabc", action.Destination)
	}
	if action.Token != "PURR:0x1" {
		t.Errorf("Token = %q, want PURR:0x1", action.Token)
	}
	if action.Amount != "3.14" {
		t.Errorf("Amount = %q, want 3.14", action.Amount)
	}
	if action.Time != 1700000000003 {
		t.Errorf("Time = %d, want 1700000000003", action.Time)
	}
}

func TestBuildApproveAgentAction(t *testing.T) {
	action := BuildApproveAgentAction("0xagent", "", 1700000000004)

	if action.Type != "approveAgent" {
		t.Errorf("Type = %q, want approveAgent", action.Type)
	}
	if action.AgentAddress != "0xagent" {
		t.Errorf("AgentAddress = %q, want 0xagent", action.AgentAddress)
	}
	if action.Nonce != 1700000000004 {
		t.Errorf("Nonce = %d, want 1700000000004", action.Nonce)
	}
}

func TestBuildUserSetAbstractionAction(t *testing.T) {
	action := BuildUserSetAbstractionAction("0xuser", "none", 1700000000005)

	if action.Type != "userSetAbstraction" {
		t.Errorf("Type = %q, want userSetAbstraction", action.Type)
	}
	if action.User != "0xuser" {
		t.Errorf("User = %q, want 0xuser", action.User)
	}
	if action.Abstraction != "none" {
		t.Errorf("Abstraction = %q, want none", action.Abstraction)
	}
	if action.Nonce != 1700000000005 {
		t.Errorf("Nonce = %d, want 1700000000005", action.Nonce)
	}
}

func TestBuildApproveAgentAction_JSONOmitEmptyName(t *testing.T) {
	action := BuildApproveAgentAction("0xagent", "", 1700000000006)
	raw, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := payload["agentName"]; ok {
		t.Fatal("agentName should be omitted when empty")
	}
}
