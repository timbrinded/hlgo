package exchange

// OrderParams holds the validated, wire-ready parameters for a single order.
type OrderParams struct {
	AssetID    int
	IsBuy      bool
	Price      string // already wire-formatted via wire.PriceToWire
	Size       string // already wire-formatted via wire.SizeToWire
	ReduceOnly bool
	Tif        string // "Gtc", "Ioc", "Alo"
	Cloid      *string
}

// TriggerParams holds optional TP/SL trigger parameters.
type TriggerParams struct {
	TriggerPx string // trigger price, wire-formatted
	Tpsl      string // "tp" or "sl"
}

// BuildOrderAction constructs an OrderAction from order parameters.
// When any order has a trigger, grouping is set to "normalTpsl"; otherwise "na".
func BuildOrderAction(orders []OrderParams, triggers []*TriggerParams, builder *BuilderInfo) *OrderAction {
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
		grouping = "normalTpsl"
	}

	return &OrderAction{
		Type:     "order",
		Orders:   wires,
		Grouping: grouping,
		Builder:  builder,
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

// BuildUpdateLeverageAction constructs an UpdateLeverageAction.
func BuildUpdateLeverageAction(assetID int, isCross bool, leverage int) *UpdateLeverageAction {
	return &UpdateLeverageAction{
		Type:     "updateLeverage",
		Asset:    assetID,
		IsCross:  isCross,
		Leverage: leverage,
	}
}

// BuildUpdateIsolatedMarginAction constructs an UpdateIsolatedMarginAction.
func BuildUpdateIsolatedMarginAction(assetID int, isBuy bool, ntli int64) *UpdateIsolatedMarginAction {
	return &UpdateIsolatedMarginAction{
		Type:  "updateIsolatedMargin",
		Asset: assetID,
		IsBuy: isBuy,
		Ntli:  ntli,
	}
}

// BuildModifyAction constructs a ModifyAction for a single order modification.
func BuildModifyAction(oid uint64, order OrderWire) *ModifyAction {
	return &ModifyAction{
		Type:  "modify",
		Oid:   oid,
		Order: order,
	}
}

// BuildScheduleCancelAction constructs a ScheduleCancelAction (dead man's switch).
func BuildScheduleCancelAction(cancelTime *int64) *ScheduleCancelAction {
	return &ScheduleCancelAction{
		Type: "scheduleCancel",
		Time: cancelTime,
	}
}

// BuildUSDClassTransferAction constructs a usdClassTransfer user-signed action.
func BuildUSDClassTransferAction(amount string, toPerp bool, nonce int64) *USDClassTransferAction {
	return &USDClassTransferAction{
		Type:   "usdClassTransfer",
		Amount: amount,
		ToPerp: toPerp,
		Nonce:  nonce,
	}
}

// BuildWithdraw3Action constructs a withdraw3 user-signed action.
func BuildWithdraw3Action(destination, amount string, time int64) *Withdraw3Action {
	return &Withdraw3Action{
		Type:        "withdraw3",
		Destination: destination,
		Amount:      amount,
		Time:        time,
	}
}

// BuildClassTransferAction constructs a classTransfer user-signed action.
func BuildClassTransferAction(amount string, toPerp bool, nonce int64) *ClassTransferAction {
	return &ClassTransferAction{
		Type:   "classTransfer",
		Amount: amount,
		ToPerp: toPerp,
		Nonce:  nonce,
	}
}

// BuildSpotSendAction constructs a spotSend user-signed action.
func BuildSpotSendAction(destination, token, amount string, time int64) *SpotSendAction {
	return &SpotSendAction{
		Type:        "spotSend",
		Destination: destination,
		Token:       token,
		Amount:      amount,
		Time:        time,
	}
}

// BuildApproveAgentAction constructs an approveAgent user-signed action.
func BuildApproveAgentAction(agentAddress, agentName string, nonce int64) *ApproveAgentAction {
	return &ApproveAgentAction{
		Type:         "approveAgent",
		AgentAddress: agentAddress,
		AgentName:    agentName,
		Nonce:        nonce,
	}
}

// BuildUserSetAbstractionAction constructs a userSetAbstraction user-signed action.
func BuildUserSetAbstractionAction(user, abstraction string, nonce int64) *UserSetAbstractionAction {
	return &UserSetAbstractionAction{
		Type:        "userSetAbstraction",
		User:        user,
		Abstraction: abstraction,
		Nonce:       nonce,
	}
}
