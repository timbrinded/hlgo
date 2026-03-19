package exchange

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/timbrinded/hlgo/pkg/output"
)

func (e *Executor) executeUserAction(
	ctx context.Context,
	action any,
	nonce int64,
	typeName string,
	typeFields []apitypes.Type,
	dryRun bool,
) (json.RawMessage, error) {
	actionMap, err := userActionMap(action)
	if err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to build action payload").
			WithDetails("cause", err.Error())
	}

	// Keep chain metadata centralized in signer behavior; this payload metadata only
	// satisfies exchange request shape and does not participate in typed hashing.
	actionMap["signatureChainId"] = userSignatureChainID
	actionMap["hyperliquidChain"] = userChain(e.mainnet)
	if dryRun {
		return json.Marshal(actionMap)
	}

	// Sign only schema fields; payload-only metadata (e.g. signatureChainId) stays out.
	signMessage := make(map[string]any, len(typeFields))
	for _, field := range typeFields {
		if field.Name == "hyperliquidChain" {
			continue
		}

		value, ok := actionMap[field.Name]
		if !ok && (typeName != "HyperliquidTransaction:ApproveAgent" || field.Name != "agentName") {
			continue
		}
		if !ok {
			// Revoke flow signs agentName="" but omits the field from the wire payload.
			value = ""
		}

		if strings.HasPrefix(field.Type, "uint") {
			switch v := value.(type) {
			case int64:
				value = strconv.FormatInt(v, 10)
			case int:
				value = strconv.Itoa(v)
			case uint64:
				value = strconv.FormatUint(v, 10)
			case uint:
				value = strconv.FormatUint(uint64(v), 10)
			}
		}
		signMessage[field.Name] = value
	}

	sig, err := e.signer.SignUserAction(typeName, typeFields, signMessage, e.mainnet)
	if err != nil {
		return nil, err
	}

	return e.client.PostExchange(ctx, actionMap, nonce, sigToWire(sig), "", nil)
}

func userActionMap(action any) (map[string]any, error) {
	switch a := action.(type) {
	case *USDClassTransferAction:
		return map[string]any{
			"type":   a.Type,
			"amount": a.Amount,
			"toPerp": a.ToPerp,
			"nonce":  a.Nonce,
		}, nil
	case *Withdraw3Action:
		return map[string]any{
			"type":        a.Type,
			"destination": a.Destination,
			"amount":      a.Amount,
			"time":        a.Time,
		}, nil
	case *ClassTransferAction:
		return map[string]any{
			"type":   a.Type,
			"amount": a.Amount,
			"toPerp": a.ToPerp,
			"nonce":  a.Nonce,
		}, nil
	case *SpotSendAction:
		return map[string]any{
			"type":        a.Type,
			"destination": a.Destination,
			"token":       a.Token,
			"amount":      a.Amount,
			"time":        a.Time,
		}, nil
	case *ApproveAgentAction:
		m := map[string]any{
			"type":         a.Type,
			"agentAddress": a.AgentAddress,
			"nonce":        a.Nonce,
		}
		if strings.TrimSpace(a.AgentName) != "" {
			m["agentName"] = a.AgentName
		}
		return m, nil
	case *UserSetAbstractionAction:
		return map[string]any{
			"type":        a.Type,
			"user":        a.User,
			"abstraction": a.Abstraction,
			"nonce":       a.Nonce,
		}, nil
	default:
		return nil, output.NewCLIError(output.ErrAPI, "unsupported user action type")
	}
}

func userChain(mainnet bool) string {
	if mainnet {
		return "Mainnet"
	}
	return "Testnet"
}
