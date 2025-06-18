// services/device/internal/utils/signing.go
package utils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// SigningKey handles firmware signing operations.
type SigningKey struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// LoadSigningKey loads a private key from file.
func LoadSigningKey(path string) (*SigningKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		privInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		priv, ok = privInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
	}

	return &SigningKey{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
	}, nil
}

// Sign creates a digital signature for the given data.
func (s *SigningKey) Sign(data []byte) (string, error) {
	hash := sha256.Sum256(data)

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify verifies a signature against data.
func (s *SigningKey) Verify(data []byte, signature string) error {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	hash := sha256.Sum256(data)

	return rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, hash[:], sig)
}
