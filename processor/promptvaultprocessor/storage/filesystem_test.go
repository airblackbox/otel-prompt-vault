// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"testing"
)

func TestFilesystemStoreAndRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	be, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFilesystemBackend: %v", err)
	}
	defer be.Close()

	data := []byte("This is a sensitive prompt content")
	ref, err := be.Store(context.Background(), "trace123", "span456", "gen_ai.input.messages", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	if ref.URI == "" {
		t.Fatal("URI should not be empty")
	}
	if ref.Checksum == "" {
		t.Fatal("Checksum should not be empty")
	}
	if ref.SizeBytes != len(data) {
		t.Fatalf("SizeBytes = %d, want %d", ref.SizeBytes, len(data))
	}

	retrieved, err := be.Retrieve(context.Background(), ref)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Fatalf("retrieved data = %q, want %q", retrieved, data)
	}
}

func TestFilesystemChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	be, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFilesystemBackend: %v", err)
	}
	defer be.Close()

	data := []byte("original content")
	ref, err := be.Store(context.Background(), "trace1", "span1", "key1", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Tamper with checksum.
	ref.Checksum = "deadbeef"
	_, err = be.Retrieve(context.Background(), ref)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestFilesystemMultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()
	be, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFilesystemBackend: %v", err)
	}
	defer be.Close()

	ref1, err := be.Store(context.Background(), "t1", "s1", "gen_ai.input.messages", []byte("prompt"))
	if err != nil {
		t.Fatalf("Store 1: %v", err)
	}

	ref2, err := be.Store(context.Background(), "t1", "s1", "gen_ai.output.messages", []byte("response"))
	if err != nil {
		t.Fatalf("Store 2: %v", err)
	}

	if ref1.URI == ref2.URI {
		t.Fatal("different keys should have different URIs")
	}

	d1, _ := be.Retrieve(context.Background(), ref1)
	d2, _ := be.Retrieve(context.Background(), ref2)

	if string(d1) != "prompt" || string(d2) != "response" {
		t.Fatal("retrieved data mismatch")
	}
}
