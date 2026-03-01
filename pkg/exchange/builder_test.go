package exchange

import "testing"

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
