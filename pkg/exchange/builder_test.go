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
