package exchange

// OrderParams holds the validated, wire-ready parameters for a single order.
type OrderParams struct {
	AssetID    int
	IsBuy      bool
	Price      string // already wire-formatted via wire.PriceToWire
	Size       string // already wire-formatted via wire.SizeToWire
	ReduceOnly bool
	Tif        string  // "Gtc", "Ioc", "Alo"
	Cloid      *string
}

// TriggerParams holds optional TP/SL trigger parameters.
type TriggerParams struct {
	TriggerPx string // trigger price, wire-formatted
	Tpsl      string // "tp" or "sl"
}

// BuildOrderAction constructs an OrderAction from order parameters.
// When any order has a trigger, grouping is set to "normalTpSl"; otherwise "na".
func BuildOrderAction(orders []OrderParams, triggers []*TriggerParams) *OrderAction {
	wires := make([]OrderWire, len(orders))
	hasTrigger := false

	for i, o := range orders {
		w := OrderWire{
			A: o.AssetID,
			B: o.IsBuy,
			P: o.Price,
			S: o.Size,
			R: o.ReduceOnly,
			C: o.Cloid,
		}

		if i < len(triggers) && triggers[i] != nil {
			hasTrigger = true
			w.T = OrderTypeWire{
				Trigger: &TriggerWire{
					IsMarket:  true,
					TriggerPx: triggers[i].TriggerPx,
					Tpsl:      triggers[i].Tpsl,
				},
			}
		} else {
			w.T = OrderTypeWire{
				Limit: &LimitTif{Tif: o.Tif},
			}
		}

		wires[i] = w
	}

	grouping := "na"
	if hasTrigger {
		grouping = "normalTpSl"
	}

	return &OrderAction{
		Type:     "order",
		Orders:   wires,
		Grouping: grouping,
	}
}

// BuildCancelAction constructs a CancelAction from cancel wires.
func BuildCancelAction(cancels []CancelWire) *CancelAction {
	return &CancelAction{
		Type:    "cancel",
		Cancels: cancels,
	}
}

// BuildCancelByCloidAction constructs a CancelByCloidAction from cancel-by-cloid wires.
func BuildCancelByCloidAction(cancels []CancelByCloidWire) *CancelByCloidAction {
	return &CancelByCloidAction{
		Type:    "cancelByCloid",
		Cancels: cancels,
	}
}
