// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package promptvaultprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr   = "promptvault"
	stability = component.StabilityLevelAlpha
)

// NewFactory returns a processor.Factory for promptvault.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Storage: StorageConfig{
			Backend: "filesystem",
			Filesystem: FilesystemConfig{
				BasePath: "/tmp/promptvault",
			},
		},
		Vault: VaultConfig{
			Keys: []string{
				"gen_ai.system_instructions",
				"gen_ai.input.messages",
				"gen_ai.output.messages",
				"gen_ai.prompt",
				"gen_ai.completion",
			},
			SizeThreshold: 0,
			Mode:          "replace_with_ref",
		},
		Crypto: CryptoConfig{
			Enable: false,
		},
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	c := cfg.(*Config)
	return newProcessor(ctx, set, c, next)
}
