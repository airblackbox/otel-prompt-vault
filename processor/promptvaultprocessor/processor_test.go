// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package promptvaultprocessor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/nostalgicskinco/prompt-vault-processor/processor/promptvaultprocessor/storage"
)

func makeTestTraces(attrs map[string]string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	return td
}

func newTestProcessor(t *testing.T, cfg *Config) (*vaultProcessor, *consumertest.TracesSink) {
	t.Helper()
	sink := &consumertest.TracesSink{}
	set := processortest.NewNopSettings()
	p, err := newProcessor(context.Background(), set, cfg, sink)
	if err != nil {
		t.Fatalf("newProcessor: %v", err)
	}
	return p, sink
}

func TestReplaceWithRef(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Storage: StorageConfig{
			Backend:    "filesystem",
			Filesystem: FilesystemConfig{BasePath: tmpDir},
		},
		Vault: VaultConfig{
			Keys:          []string{"gen_ai.input.messages"},
			SizeThreshold: 0,
			Mode:          "replace_with_ref",
		},
		Crypto: CryptoConfig{Enable: false},
	}

	p, sink := newTestProcessor(t, cfg)
	if err := p.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(context.Background())

	td := makeTestTraces(map[string]string{
		"gen_ai.input.messages": "Hello, tell me about quantum computing in detail",
		"gen_ai.model":          "gpt-4",
	})

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}

	if sink.SpanCount() != 1 {
		t.Fatalf("expected 1 span, got %d", sink.SpanCount())
	}

	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	span := spans.At(0)

	// The original attribute should now contain a vault reference JSON.
	val, ok := span.Attributes().Get("gen_ai.input.messages")
	if !ok {
		t.Fatal("expected gen_ai.input.messages attribute")
	}

	var ref storage.Reference
	if err := json.Unmarshal([]byte(val.Str()), &ref); err != nil {
		t.Fatalf("expected JSON vault reference, got: %s", val.Str())
	}

	if ref.URI == "" {
		t.Fatal("vault reference URI is empty")
	}
	if ref.Checksum == "" {
		t.Fatal("vault reference checksum is empty")
	}

	// gen_ai.model should be untouched.
	modelVal, ok := span.Attributes().Get("gen_ai.model")
	if !ok || modelVal.Str() != "gpt-4" {
		t.Fatal("gen_ai.model should be untouched")
	}
}

func TestDropMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Storage: StorageConfig{
			Backend:    "filesystem",
			Filesystem: FilesystemConfig{BasePath: tmpDir},
		},
		Vault: VaultConfig{
			Keys:          []string{"gen_ai.output.messages"},
			SizeThreshold: 0,
			Mode:          "drop",
		},
		Crypto: CryptoConfig{Enable: false},
	}

	p, sink := newTestProcessor(t, cfg)
	if err := p.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(context.Background())

	td := makeTestTraces(map[string]string{
		"gen_ai.output.messages": "This is a long AI response about quantum physics",
	})

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}

	span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	// Original key should be removed.
	_, ok := span.Attributes().Get("gen_ai.output.messages")
	if ok {
		t.Fatal("gen_ai.output.messages should have been dropped")
	}

	// Vault ref should exist.
	refVal, ok := span.Attributes().Get("gen_ai.output.messages.vault_ref")
	if !ok {
		t.Fatal("expected gen_ai.output.messages.vault_ref")
	}

	var ref storage.Reference
	if err := json.Unmarshal([]byte(refVal.Str()), &ref); err != nil {
		t.Fatalf("expected JSON vault reference: %s", refVal.Str())
	}
}

func TestKeepAndRefMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Storage: StorageConfig{
			Backend:    "filesystem",
			Filesystem: FilesystemConfig{BasePath: tmpDir},
		},
		Vault: VaultConfig{
			Keys:          []string{"gen_ai.input.messages"},
			SizeThreshold: 0,
			Mode:          "keep_and_ref",
		},
		Crypto: CryptoConfig{Enable: false},
	}

	p, sink := newTestProcessor(t, cfg)
	if err := p.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(context.Background())

	td := makeTestTraces(map[string]string{
		"gen_ai.input.messages": "What is the meaning of life?",
	})

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}

	span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	// Original should still be there.
	origVal, ok := span.Attributes().Get("gen_ai.input.messages")
	if !ok {
		t.Fatal("original attribute should be kept")
	}
	if origVal.Str() != "What is the meaning of life?" {
		t.Fatal("original value should be unchanged")
	}

	// Vault ref should also exist.
	_, ok = span.Attributes().Get("gen_ai.input.messages.vault_ref")
	if !ok {
		t.Fatal("expected vault_ref attribute")
	}
}

func TestSizeThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Storage: StorageConfig{
			Backend:    "filesystem",
			Filesystem: FilesystemConfig{BasePath: tmpDir},
		},
		Vault: VaultConfig{
			Keys:          []string{"gen_ai.input.messages"},
			SizeThreshold: 100,
			Mode:          "replace_with_ref",
		},
		Crypto: CryptoConfig{Enable: false},
	}

	p, sink := newTestProcessor(t, cfg)
	if err := p.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(context.Background())

	// Short value — should NOT be offloaded.
	td := makeTestTraces(map[string]string{
		"gen_ai.input.messages": "short",
	})

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}

	span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, ok := span.Attributes().Get("gen_ai.input.messages")
	if !ok {
		t.Fatal("attribute should still exist")
	}
	if val.Str() != "short" {
		t.Fatal("value should be unchanged because it's below threshold")
	}
}

func TestFilesystemRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Storage: StorageConfig{
			Backend:    "filesystem",
			Filesystem: FilesystemConfig{BasePath: tmpDir},
		},
		Vault: VaultConfig{
			Keys:          []string{"gen_ai.input.messages"},
			SizeThreshold: 0,
			Mode:          "replace_with_ref",
		},
		Crypto: CryptoConfig{Enable: false},
	}

	p, _ := newTestProcessor(t, cfg)
	if err := p.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(context.Background())

	// Verify file was written to disk.
	td := makeTestTraces(map[string]string{
		"gen_ai.input.messages": "stored content for round trip test",
	})

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}

	// Check that files exist in the vault directory.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected vault directory to contain trace folders")
	}

	// Walk and find the stored file.
	found := false
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("expected stored file in vault")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid filesystem config",
			cfg: Config{
				Storage: StorageConfig{Backend: "filesystem", Filesystem: FilesystemConfig{BasePath: "/tmp/test"}},
				Vault:   VaultConfig{Keys: []string{"gen_ai.input.messages"}, Mode: "replace_with_ref"},
			},
			wantErr: false,
		},
		{
			name: "invalid backend",
			cfg: Config{
				Storage: StorageConfig{Backend: "invalid"},
				Vault:   VaultConfig{Keys: []string{"gen_ai.input.messages"}, Mode: "replace_with_ref"},
			},
			wantErr: true,
		},
		{
			name: "missing base_path",
			cfg: Config{
				Storage: StorageConfig{Backend: "filesystem", Filesystem: FilesystemConfig{BasePath: ""}},
				Vault:   VaultConfig{Keys: []string{"gen_ai.input.messages"}, Mode: "replace_with_ref"},
			},
			wantErr: true,
		},
		{
			name: "no keys",
			cfg: Config{
				Storage: StorageConfig{Backend: "filesystem", Filesystem: FilesystemConfig{BasePath: "/tmp/test"}},
				Vault:   VaultConfig{Keys: []string{}, Mode: "replace_with_ref"},
			},
			wantErr: true,
		},
		{
			name: "invalid mode",
			cfg: Config{
				Storage: StorageConfig{Backend: "filesystem", Filesystem: FilesystemConfig{BasePath: "/tmp/test"}},
				Vault:   VaultConfig{Keys: []string{"gen_ai.input.messages"}, Mode: "invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
