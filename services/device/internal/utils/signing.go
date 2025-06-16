package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SigningKeyPair holds both private and public ECDSA keys for firmware signing
type SigningKeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	KeyID      string
}

// GenerateSigningKeyPair creates a new ECDSA key pair for signing firmware
func GenerateSigningKeyPair(keyID string) (*SigningKeyPair, error) {
	// Use P-256 curve (secp256r1) as specified in requirements
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA key pair: %w", err)
	}

	return &SigningKeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		KeyID:      keyID,
	}, nil
}

// SaveSigningKeyPair persists a key pair to disk in PEM format
func SaveSigningKeyPair(keyPair *SigningKeyPair, keyDir string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Marshal the private key to PKCS#8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(keyPair.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Create PEM block for private key
	privateKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Save private key to file
	privateKeyPath := filepath.Join(keyDir, fmt.Sprintf("%s.key", keyPair.KeyID))
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return fmt.Errorf("failed to encode private key to PEM: %w", err)
	}

	// Marshal the public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(keyPair.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	// Create PEM block for public key
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	// Save public key to file
	publicKeyPath := filepath.Join(keyDir, fmt.Sprintf("%s.pub", keyPair.KeyID))
	publicKeyFile, err := os.OpenFile(publicKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicKeyFile.Close()

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		return fmt.Errorf("failed to encode public key to PEM: %w", err)
	}

	return nil
}

// LoadSigningKeyPair loads an ECDSA key pair from PEM files
func LoadSigningKeyPair(keyID string, keyDir string) (*SigningKeyPair, error) {
	// Read private key file
	privateKeyPath := filepath.Join(keyDir, fmt.Sprintf("%s.key", keyID))
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	// Decode PEM block
	privateKeyBlock, _ := pem.Decode(privateKeyPEM)
	if privateKeyBlock == nil || privateKeyBlock.Type != "PRIVATE KEY" {
		return nil, errors.New("failed to decode private key PEM block")
	}

	// Parse private key
	privateKey, err := x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Ensure it's an ECDSA private key
	ecdsaPrivateKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an ECDSA key")
	}

	return &SigningKeyPair{
		PrivateKey: ecdsaPrivateKey,
		PublicKey:  &ecdsaPrivateKey.PublicKey,
		KeyID:      keyID,
	}, nil
}

// SignFirmware creates a digital signature for the given firmware file
func SignFirmware(keyPair *SigningKeyPair, firmwarePath string) (string, error) {
	// Open firmware file
	file, err := os.Open(firmwarePath)
	if err != nil {
		return "", fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()

	// Calculate SHA-256 hash of firmware
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash firmware file: %w", err)
	}
	digest := hash.Sum(nil)

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, keyPair.PrivateKey, digest)
	if err != nil {
		return "", fmt.Errorf("failed to sign firmware digest: %w", err)
	}

	// Concatenate R and S with padding to fixed length
	signature := make([]byte, 64)
	rBytes, sBytes := r.Bytes(), s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	// Return base64 encoded signature
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyFirmwareSignature verifies the signature of a firmware file
func VerifyFirmwareSignature(publicKey *ecdsa.PublicKey, firmwarePath, signature string) (bool, error) {
	// Decode base64 signature
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	if len(signatureBytes) != 64 {
		return false, errors.New("invalid signature length")
	}

	// Extract R and S from signature
	r, s := new(big.Int), new(big.Int) // Changed from ecdsa.Sign to big.Int
	r.SetBytes(signatureBytes[:32])
	s.SetBytes(signatureBytes[32:])

	// Open firmware file
	file, err := os.Open(firmwarePath)
	if err != nil {
		return false, fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()

	// Calculate SHA-256 hash of firmware
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, fmt.Errorf("failed to hash firmware file: %w", err)
	}
	digest := hash.Sum(nil)

	// Verify signature
	return ecdsa.Verify(publicKey, digest, r, s), nil
}

// GenerateDeviceSpecificKey creates a signing key specific to a device
func GenerateDeviceSpecificKey(masterKey *SigningKeyPair, deviceUID string) (*SigningKeyPair, error) {
	// Create a deterministic but unique key ID based on the device UID
	keyID := fmt.Sprintf("device-%s", deviceUID)
	
	// For real implementation, this should use a more sophisticated key derivation method
	// This is a placeholder implementation
	return GenerateSigningKeyPair(keyID)
}

// CalculateFirmwareHash returns the SHA-256 hash of a firmware file
func CalculateFirmwareHash(firmwarePath string) (string, error) {
	file, err := os.Open(firmwarePath)
	if err != nil {
		return "", fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash firmware file: %w", err)
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}