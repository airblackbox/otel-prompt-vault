// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import "context"

// Reference is the structured reference left in the span after offloading.
type Reference struct {
	// URI is the storage location (e.g., "promptvault://fs/trace_id/span_id/attr_key").
	URI string `json:"uri"`
	// Checksum is the SHA-256 hex digest of the original content.
	Checksum string `json:"checksum"`
	// Encrypted indicates whether the content is envelope-encrypted.
	Encrypted bool `json:"encrypted"`
	// SizeBytes is the original content size.
	SizeBytes int `json:"size_bytes"`
}

// Backend is the interface for vault storage implementations.
type Backend interface {
	// Store writes content to storage and returns a reference.
	Store(ctx context.Context, traceID, spanID, attrKey string, data []byte) (Reference, error)

	// Retrieve reads content back from storage by reference URI.
	Retrieve(ctx context.Context, ref Reference) ([]byte, error)

	// Close releases any resources held by the backend.
	Close() error
}
