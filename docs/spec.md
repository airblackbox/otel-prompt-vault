# Prompt Vault Processor — Specification

## Problem Statement

GenAI telemetry (prompts, completions, system instructions) is **sensitive and often large**. The OpenTelemetry GenAI semantic conventions explicitly recommend that production deployments **store content externally and record references on spans**, rather than shipping raw content to observability backends.

The OTel spec acknowledges that a collector or distribution may implement content uploading, but notes the "common approach" remains a **TODO**.

This processor fills that gap.

## Reference Object Format

When content is offloaded, the processor replaces the original attribute value with a JSON reference object:

```json
{
  "uri": "promptvault://fs/{trace_id}/{span_id}/{attr_key}",
  "checksum": "sha256_hex_digest",
  "encrypted": true,
  "size_bytes": 1234
}
```

### Fields

| Field       | Type    | Description                                           |
|-------------|---------|-------------------------------------------------------|
| `uri`       | string  | Storage location (scheme indicates backend type)      |
| `checksum`  | string  | SHA-256 hex digest of the **original** content        |
| `encrypted` | boolean | Whether the stored blob is envelope-encrypted         |
| `size_bytes`| integer | Size of the original content in bytes                 |

### URI Schemes

- `promptvault://fs/{trace_id}/{span_id}/{key}` — Filesystem backend
- `promptvault://s3/{bucket}/{prefix}{trace_id}/{span_id}/{key}` — S3-compatible backend

## Storage Layout

Objects are keyed by: `{trace_id}/{span_id}/{attribute_key}`

This enables:
- Efficient lookup by trace/span
- Bulk deletion by trace ID (retention policies)
- Correlation with trace backends

## Modes

| Mode              | Behavior                                                |
|-------------------|---------------------------------------------------------|
| `replace_with_ref`| Replace attribute value with vault reference JSON       |
| `drop`            | Remove attribute, add `{key}.vault_ref` with reference  |
| `keep_and_ref`    | Keep original value AND add `{key}.vault_ref`           |

## Encryption

Optional AES-256-GCM envelope encryption:
- Random 12-byte nonce per object
- Nonce prepended to ciphertext
- Key from environment variable or static config
- HMAC-SHA256 for metadata integrity

## Threat Model

| Threat                        | Mitigation                                     |
|-------------------------------|------------------------------------------------|
| Untrusted observability backend| Only references in traces, no content          |
| Storage compromise            | Envelope encryption (AES-256-GCM)              |
| Metadata tampering            | HMAC-SHA256 signatures                         |
| Key compromise                | Env-based key rotation; re-encrypt support     |
| Unauthorized content access   | Storage-level auth + encryption key required   |
