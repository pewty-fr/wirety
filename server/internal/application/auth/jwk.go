package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

// jwkToPublicKey converts a JWK to an RSA public key
func jwkToPublicKey(jwk map[string]interface{}) (*rsa.PublicKey, error) {
	kty, ok := jwk["kty"].(string)
	if !ok || kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", kty)
	}

	nStr, ok := jwk["n"].(string)
	if !ok {
		return nil, fmt.Errorf("missing n parameter")
	}

	eStr, ok := jwk["e"].(string)
	if !ok {
		return nil, fmt.Errorf("missing e parameter")
	}

	// Decode n (modulus)
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode n: %w", err)
	}

	// Decode e (exponent)
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode e: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
