package signer

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/timbrinded/hlgo/pkg/output"
)

// phantomAgentType defines the EIP-712 type for the phantom Agent struct.
var phantomAgentType = []apitypes.Type{
	{Name: "source", Type: "string"},
	{Name: "connectionId", Type: "bytes32"},
}

// buildConnectionID constructs the connection ID for an L1 phantom agent action.
//
// Process:
//  1. Msgpack-encode the action
//  2. Append 8-byte big-endian nonce
//  3. Append vault flag: 0x00 (no vault) or 0x01 + 20-byte address
//  4. If expiresAfter is set, append 0x00 + 8-byte big-endian timestamp
//  5. Keccak256 the result
func buildConnectionID(action any, nonce int64, vaultAddress *common.Address, expiresAfter *int64) (common.Hash, error) {
	encoded, err := msgpack.Marshal(action)
	if err != nil {
		return common.Hash{}, output.NewCLIError(output.ErrSigning, "failed to msgpack-encode action").
			WithDetails("cause", err.Error())
	}

	// Nonce: 8 bytes, big-endian, unsigned interpretation of the int64.
	var nonceBuf [8]byte
	binary.BigEndian.PutUint64(nonceBuf[:], uint64(nonce))

	// Build the payload: msgpack bytes + nonce + vault flag (+ optional expiresAfter).
	payload := make([]byte, 0, len(encoded)+8+21+9)
	payload = append(payload, encoded...)
	payload = append(payload, nonceBuf[:]...)

	if vaultAddress != nil {
		payload = append(payload, 0x01)
		payload = append(payload, vaultAddress.Bytes()...)
	} else {
		payload = append(payload, 0x00)
	}

	if expiresAfter != nil {
		var expiresAfterBuf [8]byte
		binary.BigEndian.PutUint64(expiresAfterBuf[:], uint64(*expiresAfter))
		payload = append(payload, 0x00)
		payload = append(payload, expiresAfterBuf[:]...)
	}

	return common.BytesToHash(crypto.Keccak256(payload)), nil
}

// phantomAgentTypedData builds the EIP-712 typed data for a phantom agent signing request.
func phantomAgentTypedData(source string, connectionID common.Hash) apitypes.TypedData {
	return apitypes.TypedData{Types: apitypes.Types{"EIP712Domain": eip712DomainType, "Agent": phantomAgentType}, PrimaryType: "Agent", Domain: l1Domain(), Message: apitypes.TypedDataMessage{"source": source, "connectionId": connectionID[:]}}
}
