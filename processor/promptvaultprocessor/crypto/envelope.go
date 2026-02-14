// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Envelope provides AES-GCM envelope encryption for vault content.
type Envelope struct {
	key        []byte
	hmacSecret []byte
}

// NewEnvelope creates a new envelope encryptor with a 256-bit AES key.
func NewEnvelope(hexKey string, hmacSecret string) (*Envelope, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 256 bits (32 bytes), got %d bytes", len(key))
	}
	return &Envelope{
		key:        key,
		hmacSecret: []byte(hmacSecret),
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns ciphertext (nonce prepended).
func (e *Envelope) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
func (e *Envelope) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// SignMetadata produces an HMAC-SHA256 signature for metadata integrity.
func (e *Envelope) SignMetadata(metadata string) string {
	mac := hmac.New(sha256.New, e.hmacSecret)
	mac.Write([]byte(metadata))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyMetadata checks an HMAC-SHA256 signature.
func (e *Envelope) VerifyMetadata(metadata, signature string) bool {
	expected := e.SignMetadata(metadata)
	return hmac.Equal([]byte(expected), []byte(signature))
}
