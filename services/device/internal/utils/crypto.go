package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
)

// ecdsaSignature is a helper struct to ASN.1 marshal the R and S values of a signature.
type ecdsaSignature struct {
	R, S *big.Int
}

// SigningKey holds an ECDSA private key for signing operations.
type SigningKey struct {
	privateKey *ecdsa.PrivateKey
}

// GenerateSigningKey creates a new P256 ECDSA private key.
func GenerateSigningKey() (*SigningKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return &SigningKey{privateKey: privateKey}, nil
}

// LoadSigningKey reads a PEM-encoded EC private key from a file.
func LoadSigningKey(path string) (*SigningKey, error) {
	keyData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block from key file")
	}

	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}

	return &SigningKey{privateKey: privateKey}, nil
}

// Sign calculates the SHA256 hash of the data and signs it with the private key.
// The signature is returned as a Base64-encoded string.
func (s *SigningKey) Sign(data []byte) (string, error) {
	hash := sha256.Sum256(data)

	// ecdsa.Sign returns two integers, r and s, which form the signature.
	r, sVal, err := ecdsa.Sign(rand.Reader, s.privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	// We need to marshal r and s into a single byte slice. ASN.1 is the standard format.
	signatureBytes, err := asn1.Marshal(ecdsaSignature{r, sVal})
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature: %w", err)
	}

	// Return the signature as a Base64 encoded string for easy transport.
	return base64.StdEncoding.EncodeToString(signatureBytes), nil
}
