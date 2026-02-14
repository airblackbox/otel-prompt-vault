// Copyright 2024 Nostalgic Skin Co.
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// S3Backend stores vault content in S3-compatible object storage.
type S3Backend struct {
	endpoint  string
	bucket    string
	prefix    string
	accessKey string
	secretKey string
	useSSL    bool
	client    *http.Client
}

// S3Config holds the configuration for S3 backend creation.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Prefix    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// NewS3Backend creates a new S3-compatible storage backend.
func NewS3Backend(cfg S3Config) (*S3Backend, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	return &S3Backend{
		endpoint:  cfg.Endpoint,
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
		accessKey: cfg.AccessKey,
		secretKey: cfg.SecretKey,
		useSSL:    cfg.UseSSL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Store writes data to S3-compatible storage.
func (s *S3Backend) Store(ctx context.Context, traceID, spanID, attrKey string, data []byte) (Reference, error) {
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	key := fmt.Sprintf("%s%s/%s/%s", s.prefix, traceID, spanID, attrKey)

	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return Reference{}, fmt.Errorf("failed to create S3 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(data))

	resp, err := s.client.Do(req)
	if err != nil {
		return Reference{}, fmt.Errorf("S3 PUT failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return Reference{}, fmt.Errorf("S3 PUT returned status %d: %s", resp.StatusCode, string(body))
	}

	ref := Reference{
		URI:       fmt.Sprintf("promptvault://s3/%s/%s", s.bucket, key),
		Checksum:  checksum,
		Encrypted: false,
		SizeBytes: len(data),
	}
	return ref, nil
}

// Retrieve reads content back from S3-compatible storage.
func (s *S3Backend) Retrieve(ctx context.Context, ref Reference) ([]byte, error) {
	// Extract bucket and key from URI: promptvault://s3/{bucket}/{key}
	path := ref.URI[len("promptvault://s3/"):]

	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s", scheme, s.endpoint, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("S3 GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("S3 GET returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 response: %w", err)
	}

	// Verify checksum.
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])
	if checksum != ref.Checksum {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", ref.Checksum, checksum)
	}

	return data, nil
}

// Close releases HTTP client resources.
func (s *S3Backend) Close() error {
	s.client.CloseIdleConnections()
	return nil
}
