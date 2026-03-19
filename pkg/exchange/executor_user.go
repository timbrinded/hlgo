package exchange

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/output"
)

// USDClassTransferInput holds parameters for usdClassTransfer account actions.
type USDClassTransferInput struct {
	Amount decimal.Decimal
	ToPerp bool
	DryRun bool
}

// Withdraw3Input holds parameters for withdraw3 account actions.
type Withdraw3Input struct {
	Destination string
	Amount      decimal.Decimal
	DryRun      bool
}

// ClassTransferInput holds parameters for classTransfer account actions.
type ClassTransferInput struct {
	Amount decimal.Decimal
	ToPerp bool
	DryRun bool
}

// SpotSendInput holds parameters for spotSend account actions.
type SpotSendInput struct {
	Destination string
	Token       string
	Amount      decimal.Decimal
	DryRun      bool
}

// ApproveAgentInput holds parameters for approveAgent account actions.
type ApproveAgentInput struct {
	AgentAddress string
	AgentName    string
	DryRun       bool
}

// UserSetAbstractionInput holds parameters for userSetAbstraction account actions.
type UserSetAbstractionInput struct {
	User        string
	Abstraction string
	DryRun      bool
}

const userSignatureChainID = "0x66eee"

func isSupportedUserAbstraction(abstraction string) bool {
	switch abstraction {
	case "unifiedAccount", "portfolioMargin", "disabled":
		return true
	default:
		return false
	}
}

var (
	usdClassTransferSignTypes = []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "amount", Type: "string"},
		{Name: "toPerp", Type: "bool"},
		{Name: "nonce", Type: "uint64"},
	}
	withdrawSignTypes = []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "destination", Type: "string"},
		{Name: "amount", Type: "string"},
		{Name: "time", Type: "uint64"},
	}
	spotSendSignTypes = []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "destination", Type: "string"},
		{Name: "token", Type: "string"},
		{Name: "amount", Type: "string"},
		{Name: "time", Type: "uint64"},
	}
	approveAgentSignTypes = []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "agentAddress", Type: "address"},
		{Name: "agentName", Type: "string"},
		{Name: "nonce", Type: "uint64"},
	}
	userSetAbstractionSignTypes = []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "user", Type: "address"},
		{Name: "abstraction", Type: "string"},
		{Name: "nonce", Type: "uint64"},
	}
)

// USDClassTransfer executes a usdClassTransfer user-signed action.
func (e *Executor) USDClassTransfer(ctx context.Context, input USDClassTransferInput) (json.RawMessage, error) {
	if !input.Amount.IsPositive() {
		return nil, output.NewCLIError(output.ErrValidation, "amount must be positive").
			WithDetails("value", input.Amount.String())
	}

	nonce := time.Now().UnixMilli()
	action := BuildUSDClassTransferAction(input.Amount.String(), input.ToPerp, nonce)
	return e.executeUserAction(
		ctx,
		action,
		nonce,
		"HyperliquidTransaction:UsdClassTransfer",
		usdClassTransferSignTypes,
		input.DryRun,
	)
}

// Withdraw3 executes a withdraw3 user-signed action.
func (e *Executor) Withdraw3(ctx context.Context, input Withdraw3Input) (json.RawMessage, error) {
	if !common.IsHexAddress(input.Destination) {
		return nil, output.NewCLIError(output.ErrValidation, "invalid destination address").
			WithDetails("destination", input.Destination)
	}
	if !input.Amount.IsPositive() {
		return nil, output.NewCLIError(output.ErrValidation, "amount must be positive").
			WithDetails("value", input.Amount.String())
	}

	nonce := time.Now().UnixMilli()
	action := BuildWithdraw3Action(strings.ToLower(input.Destination), input.Amount.String(), nonce)
	return e.executeUserAction(
		ctx,
		action,
		nonce,
		"HyperliquidTransaction:Withdraw",
		withdrawSignTypes,
		input.DryRun,
	)
}

// ClassTransfer executes a classTransfer user-signed action.
func (e *Executor) ClassTransfer(ctx context.Context, input ClassTransferInput) (json.RawMessage, error) {
	return e.USDClassTransfer(ctx, USDClassTransferInput(input))
}

// SpotSend executes a spotSend user-signed action.
func (e *Executor) SpotSend(ctx context.Context, input SpotSendInput) (json.RawMessage, error) {
	if !common.IsHexAddress(input.Destination) {
		return nil, output.NewCLIError(output.ErrValidation, "invalid destination address").
			WithDetails("destination", input.Destination)
	}
	if strings.TrimSpace(input.Token) == "" {
		return nil, output.NewCLIError(output.ErrValidation, "token is required")
	}
	if !input.Amount.IsPositive() {
		return nil, output.NewCLIError(output.ErrValidation, "amount must be positive").
			WithDetails("value", input.Amount.String())
	}

	nonce := time.Now().UnixMilli()
	action := BuildSpotSendAction(strings.ToLower(input.Destination), input.Token, input.Amount.String(), nonce)
	return e.executeUserAction(
		ctx,
		action,
		nonce,
		"HyperliquidTransaction:SpotSend",
		spotSendSignTypes,
		input.DryRun,
	)
}

// ApproveAgent executes an approveAgent user-signed action.
func (e *Executor) ApproveAgent(ctx context.Context, input ApproveAgentInput) (json.RawMessage, error) {
	if !common.IsHexAddress(input.AgentAddress) {
		return nil, output.NewCLIError(output.ErrValidation, "invalid agent address").
			WithDetails("agent", input.AgentAddress)
	}

	nonce := time.Now().UnixMilli()
	action := BuildApproveAgentAction(strings.ToLower(input.AgentAddress), input.AgentName, nonce)
	return e.executeUserAction(
		ctx,
		action,
		nonce,
		"HyperliquidTransaction:ApproveAgent",
		approveAgentSignTypes,
		input.DryRun,
	)
}

// UserSetAbstraction executes a userSetAbstraction user-signed action.
func (e *Executor) UserSetAbstraction(ctx context.Context, input UserSetAbstractionInput) (json.RawMessage, error) {
	if !common.IsHexAddress(input.User) {
		return nil, output.NewCLIError(output.ErrValidation, "invalid user address").
			WithDetails("user", input.User)
	}
	abstraction := strings.TrimSpace(input.Abstraction)
	if abstraction == "" {
		return nil, output.NewCLIError(output.ErrValidation, "abstraction is required")
	}
	if !isSupportedUserAbstraction(abstraction) {
		return nil, output.NewCLIError(output.ErrValidation, "unsupported abstraction value").
			WithDetails("value", abstraction).
			WithDetails("allowed", []string{"unifiedAccount", "portfolioMargin", "disabled"})
	}

	nonce := time.Now().UnixMilli()
	action := BuildUserSetAbstractionAction(strings.ToLower(input.User), abstraction, nonce)
	return e.executeUserAction(
		ctx,
		action,
		nonce,
		"HyperliquidTransaction:UserSetAbstraction",
		userSetAbstractionSignTypes,
		input.DryRun,
	)
}
