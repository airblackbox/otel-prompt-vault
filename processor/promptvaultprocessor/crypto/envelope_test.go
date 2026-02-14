// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package crypto

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	// 256-bit test key (32 bytes hex-encoded = 64 chars).
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	env, err := NewEnvelope(testKey(), "test-hmac-secret")
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}

	plaintext := []byte("This is a sensitive prompt about quantum computing")

	ciphertext, err := env.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := env.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("decrypted != plaintext: %q vs %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	env, err := NewEnvelope(testKey(), "secret")
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}

	plaintext := []byte("same input")

	ct1, _ := env.Encrypt(plaintext)
	ct2, _ := env.Encrypt(plaintext)

	if hex.EncodeToString(ct1) == hex.EncodeToString(ct2) {
		t.Fatal("two encryptions of same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	env1, _ := NewEnvelope(testKey(), "secret")
	env2, _ := NewEnvelope("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", "secret")

	ciphertext, _ := env1.Encrypt([]byte("secret data"))

	_, err := env2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("decryption with wrong key should fail")
	}
}

func TestInvalidKeyLength(t *testing.T) {
	_, err := NewEnvelope("0123456789abcdef", "secret")
	if err == nil {
		t.Fatal("should reject non-256-bit key")
	}
}

func TestHMACSignVerify(t *testing.T) {
	env, _ := NewEnvelope(testKey(), "hmac-secret-123")

	metadata := `{"uri":"promptvault://fs/abc/def/key","checksum":"aabbcc"}`
	sig := env.SignMetadata(metadata)

	if !env.VerifyMetadata(metadata, sig) {
		t.Fatal("HMAC verification should pass")
	}

	if env.VerifyMetadata(metadata+"tampered", sig) {
		t.Fatal("HMAC verification should fail for tampered metadata")
	}
}

func TestHMACDifferentSecrets(t *testing.T) {
	env1, _ := NewEnvelope(testKey(), "secret1")
	env2, _ := NewEnvelope(testKey(), "secret2")

	metadata := "test metadata"
	sig1 := env1.SignMetadata(metadata)
	sig2 := env2.SignMetadata(metadata)

	if sig1 == sig2 {
		t.Fatal("different HMAC secrets should produce different signatures")
	}
}
