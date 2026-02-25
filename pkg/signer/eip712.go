package signer

import (
	"maps"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	// chainIDL1 is the chain ID for L1 phantom agent signing.
	chainIDL1 = 1337

	// chainIDUserSigned is the chain ID for user-signed actions (Arbitrum Sepolia).
	// The Python SDK hardcodes this as 0x66eee for all user-signed actions.
	chainIDUserSigned = 421614

	// zeroAddress is the verifying contract for all Hyperliquid domains.
	zeroAddress = "0x0000000000000000000000000000000000000000"
)

// eip712DomainType is the standard EIP-712 domain type definition.
var eip712DomainType = []apitypes.Type{
	{Name: "name", Type: "string"},
	{Name: "version", Type: "string"},
	{Name: "chainId", Type: "uint256"},
	{Name: "verifyingContract", Type: "address"},
}

// l1Domain returns the EIP-712 domain for L1 phantom agent signing.
// name="Exchange", version="1", chainId=1337, verifyingContract=0x000...000.
func l1Domain() apitypes.TypedDataDomain {
	return apitypes.TypedDataDomain{
		Name:              "Exchange",
		Version:           "1",
		ChainId:           math.NewHexOrDecimal256(chainIDL1),
		VerifyingContract: zeroAddress,
	}
}

// userActionDomain returns the EIP-712 domain for user-signed actions.
// name="HyperliquidSignTransaction", version="1", chainId=421614, verifyingContract=0x000...000.
func userActionDomain() apitypes.TypedDataDomain {
	return apitypes.TypedDataDomain{
		Name:              "HyperliquidSignTransaction",
		Version:           "1",
		ChainId:           math.NewHexOrDecimal256(chainIDUserSigned),
		VerifyingContract: zeroAddress,
	}
}

// hyperliquidChain returns the chain name string for EIP-712 message content.
func hyperliquidChain(isMainnet bool) string {
	if isMainnet {
		return "Mainnet"
	}
	return "Testnet"
}

// userActionTypedData builds the EIP-712 typed data for a user-signed action.
// It automatically injects the hyperliquidChain field into the message based on isMainnet.
func userActionTypedData(typeName string, typeFields []apitypes.Type, message map[string]any, isMainnet bool) apitypes.TypedData {
	// Set hyperliquidChain in the message. The Python SDK does this inside
	// sign_user_signed_action. We make a shallow copy to avoid mutating the caller's map.
	msg := make(map[string]any, len(message)+1)
	maps.Copy(msg, message)
	msg["hyperliquidChain"] = hyperliquidChain(isMainnet)

	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": eip712DomainType,
			typeName:       typeFields,
		},
		PrimaryType: typeName,
		Domain:      userActionDomain(),
		Message:     msg,
	}
}
