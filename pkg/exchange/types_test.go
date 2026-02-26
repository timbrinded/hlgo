package exchange

import (
	"math/big"
	"strings"
	"testing"

	"github.com/timbrinded/hlgo/pkg/signer"
)

// testPrivateKey is the same test key used in signer_test.go.
const testPrivateKey = "0x0123456789012345678901234567890123456789012345678901234567890123" //nolint:gosec // test-only key

func mustDecodeHexBig(t *testing.T, s string) *big.Int {
	t.Helper()
	b, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		t.Fatalf("decoding hex big int %q failed", s)
	}
	return b
}

func assertSignature(t *testing.T, sig *signer.Signature, wantR, wantS string, wantV byte) {
	t.Helper()
	gotR := new(big.Int).SetBytes(sig.R[:])
	gotS := new(big.Int).SetBytes(sig.S[:])
	if gotR.Cmp(mustDecodeHexBig(t, wantR)) != 0 {
		t.Errorf("R mismatch\n  got:  0x%064x\n  want: %s", gotR, wantR)
	}
	if gotS.Cmp(mustDecodeHexBig(t, wantS)) != 0 {
		t.Errorf("S mismatch\n  got:  0x%064x\n  want: %s", gotS, wantS)
	}
	if sig.V != wantV {
		t.Errorf("V mismatch: got %d, want %d", sig.V, wantV)
	}
}

// TestOrderAction_SigningVector verifies that the production OrderAction type
// produces the same signature as the test types in signer_test.go.
// This is the critical cross-check that our wire types serialize identically.
func TestOrderAction_SigningVector_Mainnet(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := OrderAction{
		Type: "order",
		Orders: []OrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: OrderTypeWire{
					Limit: &LimitTif{Tif: "Gtc"},
				},
			},
		},
		Grouping: "na",
	}

	sig, err := s.SignL1Action(action, 0, nil, true)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	// These are the exact same expected values from TestSignL1Action_OrderMainnet.
	assertSignature(t, sig,
		"0xd65369825a9df5d80099e513cce430311d7d26ddf477f5b3a33d2806b100d78e",
		"0x2b54116ff64054968aa237c20ca9ff68000f977c93289157748a3162b6ea940e",
		28,
	)
}

func TestOrderAction_SigningVector_Testnet(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := OrderAction{
		Type: "order",
		Orders: []OrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: OrderTypeWire{
					Limit: &LimitTif{Tif: "Gtc"},
				},
			},
		},
		Grouping: "na",
	}

	sig, err := s.SignL1Action(action, 0, nil, false)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	assertSignature(t, sig,
		"0x82b2ba28e76b3d761093aaded1b1cdad4960b3af30212b343fb2e6cdfa4e3d54",
		"0x6b53878fc99d26047f4d7e8c90eb98955a109f44209163f52d8dc4278cbbd9f5",
		27,
	)
}

func TestOrderAction_TriggerSigningVector(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	action := OrderAction{
		Type: "order",
		Orders: []OrderWire{
			{
				A: 1,
				B: true,
				P: "100",
				S: "100",
				R: false,
				T: OrderTypeWire{
					Trigger: &TriggerWire{
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
		t.Fatalf("signing: %v", err)
	}

	assertSignature(t, sig,
		"0x98343f2b5ae8e26bb2587daad3863bc70d8792b09af1841b6fdd530a2065a3f9",
		"0x6b5bb6bb0633b710aa22b721dd9dee6d083646a5f8e581a20b545be6c1feb405",
		27,
	)
}
