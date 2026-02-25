package signer

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// testPrivateKey is a deterministic test key. DO NOT use in production.
// This is the same key used in the Hyperliquid Python SDK test vectors.
const testPrivateKey = "0x0123456789012345678901234567890123456789012345678901234567890123" //nolint:gosec // test-only key

func mustDecodeHexBig(t *testing.T, s string) *big.Int {
	t.Helper()
	if !strings.HasPrefix(s, "0x") {
		t.Fatalf("expected 0x prefix in hex big int %q", s)
	}
	b, ok := new(big.Int).SetString(s[2:], 16)
	if !ok {
		t.Fatalf("decoding hex big int %q failed", s)
	}
	return b
}

// assertSignature verifies R, S, and V of a signature against expected hex values.
func assertSignature(t *testing.T, sig *Signature, wantR, wantS string, wantV byte) {
	t.Helper()

	gotR := new(big.Int).SetBytes(sig.R[:])
	gotS := new(big.Int).SetBytes(sig.S[:])

	expectedR := mustDecodeHexBig(t, wantR)
	expectedS := mustDecodeHexBig(t, wantS)

	if gotR.Cmp(expectedR) != 0 {
		t.Errorf("R mismatch\n  got:  0x%064x\n  want: %s", gotR, wantR)
	}
	if gotS.Cmp(expectedS) != 0 {
		t.Errorf("S mismatch\n  got:  0x%064x\n  want: %s", gotS, wantS)
	}
	if sig.V != wantV {
		t.Errorf("V mismatch: got %d, want %d", sig.V, wantV)
	}
}

func TestNewSigner(t *testing.T) {
	t.Run("valid key with 0x prefix", func(t *testing.T) {
		s, err := NewSigner(testPrivateKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil signer")
		}
		addr := s.Address()
		if addr.Hex() == "" {
			t.Fatal("expected non-empty address")
		}
	})

	t.Run("valid key without 0x prefix", func(t *testing.T) {
		s, err := NewSigner("0123456789012345678901234567890123456789012345678901234567890123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil signer")
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := NewSigner("not-a-valid-key")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := NewSigner("")
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})
}

func TestSignatureHex(t *testing.T) {
	sig := &Signature{
		R: [32]byte{0xde, 0xad},
		S: [32]byte{0xbe, 0xef},
		V: 27,
	}
	h := sig.Hex()
	if h[:2] != "0x" {
		t.Errorf("expected 0x prefix, got %q", h[:2])
	}
	// 65 bytes = 130 hex chars + "0x" = 132
	if len(h) != 132 {
		t.Errorf("expected 132 chars, got %d", len(h))
	}
}

// --- L1 Action test types ---
// These structs match the Python SDK's msgpack field ordering.
// The msgpack tags control serialization order, which is critical for
// producing the same connection ID hash as the Python SDK.

type testLimitTif struct {
	Tif string `msgpack:"tif"`
}

type testOrderTypeWire struct {
	Limit   *testLimitTif    `msgpack:"limit,omitempty"`
	Trigger *testTriggerWire `msgpack:"trigger,omitempty"`
}

type testTriggerWire struct {
	IsMarket  bool   `msgpack:"isMarket"`
	TriggerPx string `msgpack:"triggerPx"`
	Tpsl      string `msgpack:"tpsl"`
}

type testOrderWire struct {
	A int               `msgpack:"a"`
	B bool              `msgpack:"b"`
	P string            `msgpack:"p"`
	S string            `msgpack:"s"`
	R bool              `msgpack:"r"`
	T testOrderTypeWire `msgpack:"t"`
	C *string           `msgpack:"c,omitempty"`
}

type testOrderAction struct {
	Type     string          `msgpack:"type"`
	Orders   []testOrderWire `msgpack:"orders"`
	Grouping string          `msgpack:"grouping"`
}

type testDummyAction struct {
	Type string `msgpack:"type"`
	Num  uint64 `msgpack:"num"`
}

// TestSignL1Action_DummyMainnet verifies the simplest Python SDK test vector.
//
// Action: {"type": "dummy", "num": float_to_int_for_hashing(1000)} = {"type": "dummy", "num": 100000000000}
// Nonce: 0, Vault: nil, Mainnet
// Expected from Python SDK:
//
//	r: 0x53749d5b30552aeb2fca34b530185976545bb22d0b3ce6f62e31be961a59298
//	s: 0x755c40ba9bf05223521753995abb2f73ab3229be8ec921f350cb447e384d8ed8
//	v: 27
func TestSignL1Action_DummyMainnet(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testDummyAction{Type: "dummy", Num: 100000000000}

	sig, err := s.SignL1Action(action, 0, nil, true)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0x53749d5b30552aeb2fca34b530185976545bb22d0b3ce6f62e31be961a59298",
		"0x755c40ba9bf05223521753995abb2f73ab3229be8ec921f350cb447e384d8ed8",
		27,
	)
}

// TestSignL1Action_DummyTestnet verifies the dummy action on testnet.
//
// Expected from Python SDK:
//
//	r: 0x542af61ef1f429707e3c76c5293c80d01f74ef853e34b76efffcb57e574f9510
//	s: 0x17b8b32f086e8cdede991f1e2c529f5dd5297cbe8128500e00cbaf766204a613
//	v: 28
func TestSignL1Action_DummyTestnet(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testDummyAction{Type: "dummy", Num: 100000000000}

	sig, err := s.SignL1Action(action, 0, nil, false)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0x542af61ef1f429707e3c76c5293c80d01f74ef853e34b76efffcb57e574f9510",
		"0x17b8b32f086e8cdede991f1e2c529f5dd5297cbe8128500e00cbaf766204a613",
		28,
	)
}

// TestSignL1Action_OrderMainnet verifies the order action on mainnet.
// This matches the Python SDK test: test_l1_action_signing_order_matches
//
// Action: order_wires_to_order_action([order_request_to_order_wire(order_request, 1)])
// Nonce: 0, Vault: nil, Mainnet
// Expected from Python SDK:
//
//	r: 0xd65369825a9df5d80099e513cce430311d7d26ddf477f5b3a33d2806b100d78e
//	s: 0x2b54116ff64054968aa237c20ca9ff68000f977c93289157748a3162b6ea940e
//	v: 28
func TestSignL1Action_OrderMainnet(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testOrderAction{
		Type: "order",
		Orders: []testOrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: testOrderTypeWire{
					Limit: &testLimitTif{Tif: "Gtc"},
				},
			},
		},
		Grouping: "na",
	}

	sig, err := s.SignL1Action(action, 0, nil, true)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0xd65369825a9df5d80099e513cce430311d7d26ddf477f5b3a33d2806b100d78e",
		"0x2b54116ff64054968aa237c20ca9ff68000f977c93289157748a3162b6ea940e",
		28,
	)
}

// TestSignL1Action_OrderTestnet verifies the order action on testnet.
// This matches the Python SDK test: test_l1_action_signing_order_matches (testnet)
//
// Expected from Python SDK:
//
//	r: 0x82b2ba28e76b3d761093aaded1b1cdad4960b3af30212b343fb2e6cdfa4e3d54
//	s: 0x6b53878fc99d26047f4d7e8c90eb98955a109f44209163f52d8dc4278cbbd9f5
//	v: 27
func TestSignL1Action_OrderTestnet(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testOrderAction{
		Type: "order",
		Orders: []testOrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: testOrderTypeWire{
					Limit: &testLimitTif{Tif: "Gtc"},
				},
			},
		},
		Grouping: "na",
	}

	sig, err := s.SignL1Action(action, 0, nil, false)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0x82b2ba28e76b3d761093aaded1b1cdad4960b3af30212b343fb2e6cdfa4e3d54",
		"0x6b53878fc99d26047f4d7e8c90eb98955a109f44209163f52d8dc4278cbbd9f5",
		27,
	)
}

// TestSignL1Action_OrderWithTrigger verifies a trigger (stop-loss) order on mainnet.
// This matches the Python SDK test: test_l1_action_signing_tpsl_order_matches
//
// Expected from Python SDK:
//
//	r: 0x98343f2b5ae8e26bb2587daad3863bc70d8792b09af1841b6fdd530a2065a3f9
//	s: 0x6b5bb6bb0633b710aa22b721dd9dee6d083646a5f8e581a20b545be6c1feb405
//	v: 27
func TestSignL1Action_OrderWithTrigger(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testOrderAction{
		Type: "order",
		Orders: []testOrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: testOrderTypeWire{
					Trigger: &testTriggerWire{
						IsMarket:  true,
						TriggerPx: "103",
						Tpsl:      "sl",
					},
				},
			},
		},
		Grouping: "na",
	}

	sig, err := s.SignL1Action(action, 0, nil, true)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0x98343f2b5ae8e26bb2587daad3863bc70d8792b09af1841b6fdd530a2065a3f9",
		"0x6b5bb6bb0633b710aa22b721dd9dee6d083646a5f8e581a20b545be6c1feb405",
		27,
	)
}

// TestSignL1Action_WithVault verifies signing with a vault address.
// This matches the Python SDK test: test_l1_action_signing_matches_with_vault
//
// Vault: 0x1719884eb866cb12b2287399b15f7db5e7d775ea
// Expected (mainnet):
//
//	r: 0x3c548db75e479f8012acf3000ca3a6b05606bc2ec0c29c50c515066a326239
//	s: 0x4d402be7396ce74fbba3795769cda45aec00dc3125a984f2a9f23177b190da2c
//	v: 28
func TestSignL1Action_WithVault(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := testDummyAction{Type: "dummy", Num: 100000000000}
	vaultAddr := common.HexToAddress("0x1719884eb866cb12b2287399b15f7db5e7d775ea")

	sig, err := s.SignL1Action(action, 0, &vaultAddr, true)
	if err != nil {
		t.Fatalf("signing L1 action: %v", err)
	}

	assertSignature(t, sig,
		"0x3c548db75e479f8012acf3000ca3a6b05606bc2ec0c29c50c515066a326239",
		"0x4d402be7396ce74fbba3795769cda45aec00dc3125a984f2a9f23177b190da2c",
		28,
	)
}

// TestSignUserAction_UsdTransfer verifies the user-signed USD transfer.
// This matches the Python SDK test: test_sign_usd_transfer_action
//
// The Python SDK calls sign_usd_transfer_action(wallet, message, False)
// which internally sets:
//   - signatureChainId = "0x66eee" (421614) for the domain
//   - hyperliquidChain = "Testnet" (because is_mainnet=False)
//
// Expected from Python SDK:
//
//	r: 0x637b37dd731507cdd24f46532ca8ba6eec616952c56218baeff04144e4a77073
//	s: 0x11a6a24900e6e314136d2592e2f8d502cd89b7c15b198e1bee043c9589f9fad7
//	v: 27
func TestSignUserAction_UsdTransfer(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	typeName := "HyperliquidTransaction:UsdSend"
	typeFields := []apitypes.Type{
		{Name: "hyperliquidChain", Type: "string"},
		{Name: "destination", Type: "string"},
		{Name: "amount", Type: "string"},
		{Name: "time", Type: "uint64"},
	}
	message := map[string]any{
		"destination": "0x5e9ee1089755c3435139848e47e6635505d5a13a",
		"amount":      "1",
		"time":        "1687816341423",
	}

	// Python SDK test uses is_mainnet=False
	sig, err := s.SignUserAction(typeName, typeFields, message, false)
	if err != nil {
		t.Fatalf("signing user action: %v", err)
	}

	assertSignature(t, sig,
		"0x637b37dd731507cdd24f46532ca8ba6eec616952c56218baeff04144e4a77073",
		"0x11a6a24900e6e314136d2592e2f8d502cd89b7c15b198e1bee043c9589f9fad7",
		27,
	)
}

func TestSignerInterface(t *testing.T) {
	// Verify LocalSigner implements Signer.
	var _ Signer = (*LocalSigner)(nil)
}

func TestSignerAddress(t *testing.T) {
	s, err := NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	// The address for the test key is well-known.
	expected := "0x14791697260E4c9A71f18484C9f997B308e59325"
	got := s.Address().Hex()
	if got != expected {
		t.Errorf("address mismatch: got %s, want %s", got, expected)
	}
}
