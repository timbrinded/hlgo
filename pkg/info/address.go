package info

import (
	"os"
	"regexp"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/signer"
)

// ethAddrRegex matches a 0x-prefixed, 40 hex char Ethereum address.
var ethAddrRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// ResolveUserAddress resolves an Ethereum address for info queries.
//
// Priority:
//  1. Explicit address argument (validated for 0x + 40 hex chars)
//  2. Wallet derived from config's private key env var
//  3. Error if neither is available
func ResolveUserAddress(explicitAddr string, cfg *config.Config) (string, error) {
	if explicitAddr != "" {
		if !ethAddrRegex.MatchString(explicitAddr) {
			return "", output.NewCLIError(output.ErrValidation, "invalid Ethereum address format").
				WithDetails("address", explicitAddr).
				WithDetails("hint", "expected 0x prefix followed by 40 hex characters")
		}
		return explicitAddr, nil
	}

	keyHex := os.Getenv(cfg.PrivateKeyEnv)
	if keyHex == "" {
		return "", output.NewCLIError(output.ErrConfig, "no address available: provide --address or set "+cfg.PrivateKeyEnv).
			WithDetails("private_key_env", cfg.PrivateKeyEnv)
	}

	s, err := signer.NewSigner(keyHex)
	if err != nil {
		return "", err
	}
	return s.Address().Hex(), nil
}
