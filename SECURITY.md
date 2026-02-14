# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in the Prompt Vault Processor, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, email: **jason.j.shotwell@gmail.com**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact assessment
- Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Security Model

The Prompt Vault Processor handles sensitive GenAI content (prompts, completions, system instructions). Its security model includes:

### Data at Rest
- Optional AES-256-GCM envelope encryption for all stored content
- HMAC-SHA256 integrity signing for metadata
- Configurable key management (environment variable or static key; KMS support planned)

### Data in Transit
- Content is offloaded before traces leave the collector pipeline
- Only structured references (URI + checksum + encryption flag) remain in traces
- No plaintext content is forwarded to observability backends

### Access Control
- Vault content retrieval requires storage backend credentials
- Encrypted content additionally requires the decryption key
- RBAC integration planned for enterprise deployments

### Threat Model
- **Untrusted observability backend**: Traces contain references, not content
- **Storage compromise**: Encrypted content cannot be read without the key
- **Metadata tampering**: HMAC signatures detect unauthorized modifications
- **Key compromise**: Rotate keys via environment variable; re-encrypt stored content

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.x     | Current development — security patches applied promptly |

## Responsible Disclosure

We follow responsible disclosure practices. Security researchers who report vulnerabilities will be credited in the release notes (unless they prefer anonymity).
