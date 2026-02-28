// Package signer implements EIP-712 typed data signing for Hyperliquid L1 and user-signed actions.
package signer

import (
	"crypto/ecdsa"
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/timbrinded/hlgo/pkg/output"
)

// Signer signs actions for the Hyperliquid exchange.
type Signer interface {
	// SignL1Action signs a trading action using the L1 phantom agent path.
	// action is msgpack-encoded, combined with nonce and vault info, then hashed.
	SignL1Action(action any, nonce int64, vaultAddress *common.Address, expiresAfter *int64, isMainnet bool) (*Signature, error)

	// SignUserAction signs an account operation using direct EIP-712 signing.
	SignUserAction(typeName string, typeFields []apitypes.Type, message map[string]any, isMainnet bool) (*Signature, error)

	// Address returns the Ethereum address derived from the private key.
	Address() common.Address
}

// Signature holds the components of an ECDSA signature.
type Signature struct {
	R [32]byte
	S [32]byte
	V byte
}

// Hex returns the concatenated signature as a hex string with 0x prefix.
func (s *Signature) Hex() string {
	var buf [65]byte
	copy(buf[:32], s.R[:])
	copy(buf[32:64], s.S[:])
	buf[64] = s.V
	return "0x" + hex.EncodeToString(buf[:])
}

// LocalSigner signs actions using a local ECDSA private key.
type LocalSigner struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// NewSigner creates a new LocalSigner from a hex-encoded private key.
// The key may optionally include a "0x" prefix.
func NewSigner(privateKeyHex string) (*LocalSigner, error) {
	// Strip 0x prefix if present — crypto.HexToECDSA expects raw hex.
	cleaned := strings.TrimPrefix(privateKeyHex, "0x")

	key, err := crypto.HexToECDSA(cleaned)
	if err != nil {
		// Never include the key material in the error message.
		return nil, output.NewCLIError(output.ErrConfig, "invalid private key format").
			WithDetails("cause", err.Error())
	}

	address := crypto.PubkeyToAddress(key.PublicKey)

	return &LocalSigner{
		key:     key,
		address: address,
	}, nil
}

// Address returns the Ethereum address derived from the private key.
func (s *LocalSigner) Address() common.Address {
	return s.address
}

// SignL1Action signs a trading action using the L1 phantom agent path.
//
// Flow:
//  1. Msgpack-encode the action
//  2. Append 8-byte big-endian nonce (millisecond timestamp)
//  3. Append vault address flag: 0x00 (no vault) or 0x01 + 20-byte address
//  4. If expiresAfter is set, append 0x00 + 8-byte big-endian timestamp
//  5. Keccak256 hash the combined bytes → connectionId
//  5. Build phantom Agent struct: {source: "a"|"b", connectionId: <hash>}
//  6. EIP-712 sign the Agent struct
func (s *LocalSigner) SignL1Action(action any, nonce int64, vaultAddress *common.Address, expiresAfter *int64, isMainnet bool) (*Signature, error) {
	connectionID, err := buildConnectionID(action, nonce, vaultAddress, expiresAfter)
	if err != nil {
		return nil, err // already a structured CLIError from buildConnectionID
	}

	source := "b" // testnet
	if isMainnet {
		source = "a" // mainnet
	}

	typedData := phantomAgentTypedData(source, connectionID)

	return signTypedData(s.key, typedData)
}

// SignUserAction signs an account operation using direct EIP-712 signing.
//
// Used for: transfers, withdrawals, agent management.
// Chain ID: 421614 (Arbitrum Sepolia, hardcoded per Python SDK as 0x66eee).
// Domain: name="HyperliquidSignTransaction", version="1".
// The hyperliquidChain field is automatically injected into the message.
func (s *LocalSigner) SignUserAction(typeName string, typeFields []apitypes.Type, message map[string]any, isMainnet bool) (*Signature, error) {
	typedData := userActionTypedData(typeName, typeFields, message, isMainnet)

	return signTypedData(s.key, typedData)
}

// signTypedData computes the EIP-712 hash and signs it with the given key.
func signTypedData(key *ecdsa.PrivateKey, typedData apitypes.TypedData) (*Signature, error) {
	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, output.NewCLIError(output.ErrSigning, "failed to hash typed data").
			WithDetails("cause", err.Error())
	}

	// crypto.Sign returns 65 bytes: [R(32) || S(32) || V(1)] where V is 0 or 1.
	sigBytes, err := crypto.Sign(hash, key)
	if err != nil {
		return nil, output.NewCLIError(output.ErrSigning, "ECDSA signing failed").
			WithDetails("cause", err.Error())
	}

	var sig Signature
	copy(sig.R[:], sigBytes[:32])
	copy(sig.S[:], sigBytes[32:64])
	// Ethereum convention: V = recovery_id + 27
	sig.V = sigBytes[64] + 27

	return &sig, nil
}
