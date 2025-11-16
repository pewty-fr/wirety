package wireguard

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair generates a WireGuard private/public key pair
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// Generate 32 random bytes for private key
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", err
	}

	// Clamp the private key according to Curve25519 requirements
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	// Derive public key from private key using Curve25519
	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)

	// Encode keys to base64
	privateKey = base64.StdEncoding.EncodeToString(private[:])
	publicKey = base64.StdEncoding.EncodeToString(public[:])

	return privateKey, publicKey, nil
}

// GeneratePrivateKey generates only a WireGuard private key
func GeneratePrivateKey() (string, error) {
	privateKey, _, err := GenerateKeyPair()
	return privateKey, err
}

// DerivePublicKey derives the public key from a private key
func DerivePublicKey(privateKey string) (string, error) {
	// Decode private key from base64
	private, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}

	// Ensure it's 32 bytes
	if len(private) != 32 {
		return "", err
	}

	// Derive public key using Curve25519
	var privateArray, publicArray [32]byte
	copy(privateArray[:], private)
	curve25519.ScalarBaseMult(&publicArray, &privateArray)

	// Encode to base64
	publicKey := base64.StdEncoding.EncodeToString(publicArray[:])
	return publicKey, nil
}

// GeneratePresharedKey generates a WireGuard preshared key
func GeneratePresharedKey() (string, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(key[:]), nil
}
