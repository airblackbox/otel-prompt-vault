// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package promptvaultprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/nostalgicskinco/prompt-vault-processor/processor/promptvaultprocessor/crypto"
	"github.com/nostalgicskinco/prompt-vault-processor/processor/promptvaultprocessor/storage"
)

type vaultProcessor struct {
	cfg      *Config
	logger   *zap.Logger
	next     consumer.Traces
	backend  storage.Backend
	envelope *crypto.Envelope
	keySet   map[string]bool
}

func newProcessor(
	_ context.Context,
	set processor.Settings,
	cfg *Config,
	next consumer.Traces,
) (*vaultProcessor, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Build key lookup set.
	keySet := make(map[string]bool, len(cfg.Vault.Keys))
	for _, k := range cfg.Vault.Keys {
		keySet[k] = true
	}

	p := &vaultProcessor{
		cfg:    cfg,
		logger: set.Logger,
		next:   next,
		keySet: keySet,
	}

	return p, nil
}

// Start initializes the storage backend and optional encryption.
func (p *vaultProcessor) Start(_ context.Context, _ component.Host) error {
	// Initialize storage backend.
	switch p.cfg.Storage.Backend {
	case "filesystem":
		be, err := storage.NewFilesystemBackend(p.cfg.Storage.Filesystem.BasePath)
		if err != nil {
			return fmt.Errorf("failed to init filesystem backend: %w", err)
		}
		p.backend = be
	case "s3":
		be, err := storage.NewS3Backend(storage.S3Config{
			Endpoint:  p.cfg.Storage.S3.Endpoint,
			Bucket:    p.cfg.Storage.S3.Bucket,
			Prefix:    p.cfg.Storage.S3.Prefix,
			Region:    p.cfg.Storage.S3.Region,
			AccessKey: p.cfg.Storage.S3.AccessKey,
			SecretKey: p.cfg.Storage.S3.SecretKey,
			UseSSL:    p.cfg.Storage.S3.UseSSL,
		})
		if err != nil {
			return fmt.Errorf("failed to init S3 backend: %w", err)
		}
		p.backend = be
	}

	// Initialize encryption if enabled.
	if p.cfg.Crypto.Enable {
		hexKey := p.cfg.Crypto.StaticKey
		if p.cfg.Crypto.KeySource == "env" {
			hexKey = os.Getenv(p.cfg.Crypto.EnvVar)
		}
		env, err := crypto.NewEnvelope(hexKey, p.cfg.Crypto.HMACSecret)
		if err != nil {
			return fmt.Errorf("failed to init encryption: %w", err)
		}
		p.envelope = env
	}

	p.logger.Info("Prompt vault processor started",
		zap.String("backend", p.cfg.Storage.Backend),
		zap.String("mode", p.cfg.Vault.Mode),
		zap.Bool("encryption", p.cfg.Crypto.Enable),
		zap.Int("keys", len(p.cfg.Vault.Keys)),
	)
	return nil
}

// Shutdown releases resources.
func (p *vaultProcessor) Shutdown(_ context.Context) error {
	if p.backend != nil {
		return p.backend.Close()
	}
	return nil
}

// Capabilities indicates this processor mutates data.
func (p *vaultProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeTraces processes traces, offloading matching attributes to vault storage.
func (p *vaultProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		scopeSpans := rs.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				p.processSpan(ctx, span)
				p.processSpanEvents(ctx, span)
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

// processSpan offloads matching attributes from a span.
func (p *vaultProcessor) processSpan(ctx context.Context, span ptrace.Span) {
	traceID := span.TraceID().String()
	spanID := span.SpanID().String()

	type pendingOp struct {
		key string
		ref storage.Reference
	}
	var ops []pendingOp

	span.Attributes().Range(func(k string, v pcommon.Value) bool {
		if !p.keySet[k] {
			return true
		}

		val := v.Str()
		if len(val) < p.cfg.Vault.SizeThreshold {
			return true
		}

		data := []byte(val)

		// Encrypt if enabled.
		if p.envelope != nil {
			encrypted, err := p.envelope.Encrypt(data)
			if err != nil {
				p.logger.Error("encryption failed", zap.String("key", k), zap.Error(err))
				return true
			}
			data = encrypted
		}

		ref, err := p.backend.Store(ctx, traceID, spanID, k, data)
		if err != nil {
			p.logger.Error("vault store failed", zap.String("key", k), zap.Error(err))
			return true
		}

		if p.envelope != nil {
			ref.Encrypted = true
		}

		ops = append(ops, pendingOp{key: k, ref: ref})
		return true
	})

	for _, op := range ops {
		refJSON, _ := json.Marshal(op.ref)

		switch p.cfg.Vault.Mode {
		case "replace_with_ref":
			span.Attributes().PutStr(op.key, string(refJSON))
		case "drop":
			span.Attributes().Remove(op.key)
			span.Attributes().PutStr(op.key+".vault_ref", string(refJSON))
		case "keep_and_ref":
			span.Attributes().PutStr(op.key+".vault_ref", string(refJSON))
		}
	}
}

// processSpanEvents offloads matching attributes from span events.
func (p *vaultProcessor) processSpanEvents(ctx context.Context, span ptrace.Span) {
	traceID := span.TraceID().String()
	spanID := span.SpanID().String()

	events := span.Events()
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)

		type pendingOp struct {
			key string
			ref storage.Reference
		}
		var ops []pendingOp

		event.Attributes().Range(func(k string, v pcommon.Value) bool {
			if !p.keySet[k] {
				return true
			}

			val := v.Str()
			if len(val) < p.cfg.Vault.SizeThreshold {
				return true
			}

			data := []byte(val)

			if p.envelope != nil {
				encrypted, err := p.envelope.Encrypt(data)
				if err != nil {
					p.logger.Error("encryption failed (event)", zap.String("key", k), zap.Error(err))
					return true
				}
				data = encrypted
			}

			eventKey := fmt.Sprintf("%s/event_%d/%s", spanID, i, k)
			ref, err := p.backend.Store(ctx, traceID, spanID, eventKey, data)
			if err != nil {
				p.logger.Error("vault store failed (event)", zap.String("key", k), zap.Error(err))
				return true
			}

			if p.envelope != nil {
				ref.Encrypted = true
			}

			ops = append(ops, pendingOp{key: k, ref: ref})
			return true
		})

		for _, op := range ops {
			refJSON, _ := json.Marshal(op.ref)

			switch p.cfg.Vault.Mode {
			case "replace_with_ref":
				event.Attributes().PutStr(op.key, string(refJSON))
			case "drop":
				event.Attributes().Remove(op.key)
				event.Attributes().PutStr(op.key+".vault_ref", string(refJSON))
			case "keep_and_ref":
				event.Attributes().PutStr(op.key+".vault_ref", string(refJSON))
			}
		}
	}
}
