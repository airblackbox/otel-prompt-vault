# Prompt Vault Processor

**The missing piece in GenAI observability: offload sensitive content, keep traceability.**

An OpenTelemetry Collector processor that uploads sensitive GenAI content (prompts, completions, system instructions) to external storage and leaves structured references in traces — with optional envelope encryption and RBAC-ready access patterns.

## Why This Exists

The OpenTelemetry GenAI semantic conventions explicitly recommend that production deployments **store content externally and record references on spans**. The spec acknowledges that collectors may implement content uploading, but the "common approach" remains undefined.

The upstream "Blob Upload Processor" proposal was closed as "not planned," leaving clear whitespace for a purpose-built solution.

**This processor fills that gap.**

## How It Works

```
GenAI App → OTel Collector → [Prompt Vault Processor] → Observability Backend
                                      ↓
                              Object Storage (encrypted)
```

1. GenAI spans arrive with opt-in content attributes (`gen_ai.input.messages`, `gen_ai.output.messages`, etc.)
2. The processor offloads matched content to external storage (filesystem or S3-compatible)
3. Traces continue downstream with **structured references** instead of raw content
4. Authorized users retrieve and decrypt content via the `promptvaultctl` CLI

## Key Features

- **Drop-in OTel Collector processor** — no SDK changes required
- **Three offload modes**: `replace_with_ref`, `drop`, `keep_and_ref`
- **AES-256-GCM envelope encryption** with HMAC metadata integrity
- **Size threshold filtering** — only offload content above N bytes
- **Filesystem and S3-compatible storage** backends
- **Content retrieval CLI** (`promptvaultctl`) with decryption support
- **Checksum verification** (SHA-256) on every retrieval

## Quick Start

### 5-Minute Local Demo

```bash
# Clone and build
git clone https://github.com/nostalgicskinco/prompt-vault-processor.git
cd prompt-vault-processor
go build ./...

# Run with docker-compose (includes MinIO for S3 storage)
cd examples/docker-compose
docker-compose up -d

# Send test traces with GenAI attributes
# (traces arrive on localhost:4317 via OTLP gRPC)

# View vault contents
promptvaultctl get '{"uri":"promptvault://s3/...","checksum":"...","encrypted":true}'
```

### Minimal Collector Config

```yaml
processors:
  promptvault:
    storage:
      backend: filesystem
      filesystem:
        base_path: /var/lib/promptvault
    vault:
      keys:
        - gen_ai.system_instructions
        - gen_ai.input.messages
        - gen_ai.output.messages
      size_threshold: 0
      mode: replace_with_ref
    crypto:
      enable: true
      key_source: env
      env_var: PROMPTVAULT_KEY
```

## Configuration Reference

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `storage.backend` | string | `filesystem` | Storage type: `filesystem` or `s3` |
| `storage.filesystem.base_path` | string | `/tmp/promptvault` | Directory for filesystem storage |
| `storage.s3.endpoint` | string | — | S3-compatible endpoint URL |
| `storage.s3.bucket` | string | — | S3 bucket name |
| `storage.s3.prefix` | string | — | Key prefix within bucket |
| `vault.keys` | []string | GenAI content keys | Attribute keys to match for offloading |
| `vault.size_threshold` | int | `0` | Min bytes before offloading (0 = always) |
| `vault.mode` | string | `replace_with_ref` | Offload mode |
| `crypto.enable` | bool | `false` | Enable AES-256-GCM envelope encryption |
| `crypto.key_source` | string | — | Key source: `env` or `static` |
| `crypto.env_var` | string | — | Env var with hex-encoded 256-bit key |

## Vault Reference Format

Offloaded attributes are replaced with a JSON reference:

```json
{
  "uri": "promptvault://fs/{trace_id}/{span_id}/{attr_key}",
  "checksum": "sha256_hex_digest",
  "encrypted": true,
  "size_bytes": 1234
}
```

## Architecture

This processor operates at the **collector boundary** — the recommended governance point for OpenTelemetry pipelines. Content never reaches your observability backend; only lightweight references do.

### Security Model

| Layer | Protection |
|-------|-----------|
| **Transit** | Content removed before traces leave collector |
| **Storage** | AES-256-GCM envelope encryption |
| **Integrity** | SHA-256 checksums + HMAC-SHA256 metadata signing |
| **Access** | Storage credentials + encryption key required |

## Roadmap

- [ ] KMS/HSM key management integration
- [ ] RBAC integration (Okta/SCIM)
- [ ] Presigned URL mode for storage-native auth
- [ ] Retention policies and legal hold
- [ ] Helm chart for Kubernetes deployment
- [ ] Grafana dashboard for vault metrics
- [ ] Event-level content offloading (span events)
- [ ] Content search and indexing (enterprise)

## License

This project is dual-licensed:

- **AGPL-3.0** for open-source and internal non-commercial use ([LICENSE](LICENSE))
- **Commercial license** for hosted services, SaaS platforms, and commercial products ([COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md))

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributor license agreement details.

## Part of the GenAI Infrastructure Standards Portfolio

| Project | Description |
|---------|-------------|
| **[Prompt Vault Processor](https://github.com/nostalgicskinco/prompt-vault-processor)** | Content offload + encryption for GenAI traces |
| [GenAI Safe Processor](https://github.com/nostalgicskinco/opentelemetry-collector-processor-genai) | Privacy-by-default redaction + cost metrics + loop detection |
| [GenAI Semantic Normalizer](https://github.com/nostalgicskinco/genai-semantic-normalizer) | One schema to query all LLM traces |
