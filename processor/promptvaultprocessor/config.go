// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package promptvaultprocessor

import (
	"fmt"
)

// Config holds the configuration for the prompt vault processor.
type Config struct {
	// Storage configures where offloaded content is stored.
	Storage StorageConfig `mapstructure:"storage"`

	// Vault configures which attributes to offload and how.
	Vault VaultConfig `mapstructure:"vault"`

	// Crypto configures optional envelope encryption.
	Crypto CryptoConfig `mapstructure:"crypto"`
}

// StorageConfig configures the storage backend.
type StorageConfig struct {
	// Backend is the storage type: "filesystem" or "s3".
	Backend string `mapstructure:"backend"`

	// Filesystem settings (used when backend = "filesystem").
	Filesystem FilesystemConfig `mapstructure:"filesystem"`

	// S3 settings (used when backend = "s3").
	S3 S3Config `mapstructure:"s3"`
}

// FilesystemConfig holds settings for local filesystem storage.
type FilesystemConfig struct {
	// BasePath is the directory where vault objects are written.
	BasePath string `mapstructure:"base_path"`
}

// S3Config holds settings for S3-compatible object storage.
type S3Config struct {
	// Endpoint is the S3-compatible endpoint URL (e.g., http://localhost:9000).
	Endpoint string `mapstructure:"endpoint"`
	// Bucket name.
	Bucket string `mapstructure:"bucket"`
	// Prefix (key prefix within the bucket).
	Prefix string `mapstructure:"prefix"`
	// Region for AWS S3.
	Region string `mapstructure:"region"`
	// AccessKey for authentication.
	AccessKey string `mapstructure:"access_key"`
	// SecretKey for authentication.
	SecretKey string `mapstructure:"secret_key"`
	// UseSSL enables HTTPS for the connection.
	UseSSL bool `mapstructure:"use_ssl"`
}

// VaultConfig configures which span attributes to offload.
type VaultConfig struct {
	// Keys lists the attribute keys to match for offloading.
	Keys []string `mapstructure:"keys"`

	// SizeThreshold is the minimum size in bytes before offloading.
	// Values smaller than this are left in place.
	SizeThreshold int `mapstructure:"size_threshold"`

	// Mode controls how matched attributes are handled:
	// "replace_with_ref" - replace value with a vault reference URI
	// "drop"             - remove the attribute entirely, store in vault
	// "keep_and_ref"     - keep original AND add a vault reference attribute
	Mode string `mapstructure:"mode"`
}

// CryptoConfig configures optional envelope encryption for stored content.
type CryptoConfig struct {
	// Enable turns on envelope encryption.
	Enable bool `mapstructure:"enable"`

	// KeySource is how the encryption key is provided: "env" or "static".
	KeySource string `mapstructure:"key_source"`

	// StaticKey is a hex-encoded 256-bit AES key (used when key_source = "static").
	StaticKey string `mapstructure:"static_key"`

	// EnvVar is the environment variable name containing the hex-encoded key
	// (used when key_source = "env").
	EnvVar string `mapstructure:"env_var"`

	// HMACSecret is used for metadata integrity signing.
	HMACSecret string `mapstructure:"hmac_secret"`
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	switch c.Storage.Backend {
	case "filesystem", "s3":
		// ok
	default:
		return fmt.Errorf("unsupported storage backend: %q (must be filesystem or s3)", c.Storage.Backend)
	}

	if c.Storage.Backend == "filesystem" && c.Storage.Filesystem.BasePath == "" {
		return fmt.Errorf("filesystem.base_path is required when backend is filesystem")
	}

	if c.Storage.Backend == "s3" && c.Storage.S3.Bucket == "" {
		return fmt.Errorf("s3.bucket is required when backend is s3")
	}

	switch c.Vault.Mode {
	case "replace_with_ref", "drop", "keep_and_ref":
		// ok
	default:
		return fmt.Errorf("unsupported vault mode: %q", c.Vault.Mode)
	}

	if len(c.Vault.Keys) == 0 {
		return fmt.Errorf("vault.keys must contain at least one attribute key")
	}

	if c.Crypto.Enable {
		switch c.Crypto.KeySource {
		case "env", "static":
			// ok
		default:
			return fmt.Errorf("unsupported crypto key_source: %q", c.Crypto.KeySource)
		}
	}

	return nil
}
