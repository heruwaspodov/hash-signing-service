# hash-signing-service

A stateless microservice that performs RSA digital signing for PAdES (PDF Advanced Electronic Signatures). It receives a **pre-computed base64 digest**, signs it with an RSA private key using PKCS#1 v1.5 padding, and returns the base64-encoded signature.

This service is designed to be called by a backend orchestrator (e.g. `msign-backend`) that handles PDF manipulation, CMS structure, and certificate embedding. This service only performs the raw RSA operation — no PDF handling, no database, no session.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21.5 |
| HTTP Router | [gorilla/mux](https://github.com/gorilla/mux) v1.8.1 |
| Config | [joho/godotenv](https://github.com/joho/godotenv) v1.5.1 |
| Cryptography | Go stdlib (`crypto/rsa`, `crypto/x509`, `encoding/pem`) |

---

## Prerequisites

- Go 1.21.5+
- RSA private key + certificate chain in PEM format (signing key, signing cert, sub-CA, root-CA)

---

## Installation

**1. Clone the repository**

```bash
git clone <repo-url>
cd hash-signing-service
```

**2. Install dependencies**

```bash
go mod download
```

**3. Setup environment**

```bash
cp .env.example .env
```

Edit `.env` as needed:

```dotenv
APP_ENV="local"
APP_LANG="en"
APP_PORT="7777"
APP_TIMEZONE="Asia/Jakarta"
APP_SECRET_KEY=thisisaverysecretkeywith32chars!

ENABLE_CORS=true
ENABLE_LOGGER=true

CERT_FILE=certs/signing.crt
CERT_KEY_FILE=certs/signing.key
CERT_SUB_CA_FILE=certs/sub-ca.crt
CERT_ROOT_CA_FILE=certs/root-ca.crt
```

> The `certs/` directory already contains a sample certificate chain for local development.

**4. Run**

```bash
go run main.go
```

Service will start on `http://localhost:7777`.

---

## API Reference

### Health Check

```
GET /
```

**Response `200 OK`**

```json
{
  "status": "ok"
}
```

---

### Sign Hash

Signs one or more pre-computed digests with the RSA private key. Digests must **not** be re-hashed — pass the raw digest bytes as base64. Signatures are returned in the same order as the input hashes.

```
POST /api/v1/hash-sign
Content-Type: application/json
```

**Request Body**

| Field | Type | Required | Description |
|---|---|---|---|
| `hash` | `string[]` | Yes | One or more base64-encoded pre-computed digests. `signatures[i]` corresponds to `hash[i]`. |
| `hash_algo` | `string` | Yes | OID of the digest algorithm used to produce the hash |
| `sign_algo` | `string` | Yes | OID of the signing algorithm |

**Supported OIDs**

| Field | OID | Algorithm |
|---|---|---|
| `hash_algo` | `2.16.840.1.101.3.4.2.1` | SHA-256 |
| `hash_algo` | `2.16.840.1.101.3.4.2.2` | SHA-384 |
| `hash_algo` | `2.16.840.1.101.3.4.2.3` | SHA-512 |
| `sign_algo` | `1.2.840.113549.1.1.11` | sha256WithRSAEncryption |

**Sample Request — single hash**

```bash
HASH=$(echo -n "signed attributes content" | openssl dgst -sha256 -binary | base64)

curl -sS -X POST http://localhost:7777/api/v1/hash-sign \
  -H 'Content-Type: application/json' \
  -d "{
    \"hash\":      [\"$HASH\"],
    \"hash_algo\": \"2.16.840.1.101.3.4.2.1\",
    \"sign_algo\":  \"1.2.840.113549.1.1.11\"
  }"
```

**Sample Request — multiple hashes**

```bash
HASH1=$(echo -n "first document digest" | openssl dgst -sha256 -binary | base64)
HASH2=$(echo -n "second document digest" | openssl dgst -sha256 -binary | base64)

curl -sS -X POST http://localhost:7777/api/v1/hash-sign \
  -H 'Content-Type: application/json' \
  -d "{
    \"hash\":      [\"$HASH1\", \"$HASH2\"],
    \"hash_algo\": \"2.16.840.1.101.3.4.2.1\",
    \"sign_algo\":  \"1.2.840.113549.1.1.11\"
  }"
```

**Response `200 OK`**

```json
{
  "signatures": [
    "mPCPiOvYwtz5B8haTkd9o7l+TzvEYoG9Y4uEofaiR9pyT7enQRzH44+0MJ7udLOV...",
    "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890abcdefghijklmnopqrstuvwxyz..."
  ]
}
```

> `signatures[i]` always corresponds to `hash[i]`.

**Error Responses**

| Cause | HTTP | `error.code` |
|---|---|---|
| Missing or blank required field | `400` | `bad_request` |
| Unsupported `hash_algo` OID | `400` | `bad_request` |
| Invalid base64 in `hash` | `400` | `bad_request` |
| Any other failure | `500` | `internal_error` |

**Response `400 Bad Request`**

```json
{
  "error": {
    "code": "bad_request",
    "message": "missing required params"
  }
}
```

**Response `500 Internal Server Error`**

```json
{
  "error": {
    "code": "internal_error",
    "message": "signing failed"
  }
}
```

---

## Signer Backend

The service supports two signing backends, switchable via `SIGNER_BACKEND` env — no code changes required.

| `SIGNER_BACKEND` | Backend | When to use |
|---|---|---|
| `file` (default) | PEM private key from disk | Local development, CI |
| `pkcs11` | PKCS#11 HSM (SoftHSM2 / CloudHSM) | Staging, production |

### Architecture

```
handler → RawHashSignService → Signer interface
                                    ├── FileSigner   (SIGNER_BACKEND=file)
                                    └── HSMSigner    (SIGNER_BACKEND=pkcs11)
```

Swapping backends only requires changing env vars — handler and service layer are unaffected.

---

### SIGNER_BACKEND=file (default)

```dotenv
SIGNER_BACKEND=file
CERT_KEY_FILE=certs/signing.key
```

No additional setup needed. Service loads the PEM private key at startup.

---

### SIGNER_BACKEND=pkcs11

Uses PKCS#11 via [miekg/pkcs11](https://github.com/miekg/pkcs11). Compatible with **SoftHSM2** (dev) and **AWS CloudHSM** — both expose a native PKCS#11 shared library (`.so`/`.dylib`).

> **GCP Cloud KMS and Azure Key Vault** use REST/SDK APIs, not a PKCS#11 module. They are **not** drop-in compatible with this backend. Integrating them requires a separate `Signer` implementation using their respective Go SDKs (`cloud.google.com/go/kms`, `github.com/Azure/azure-sdk-for-go`).

```dotenv
SIGNER_BACKEND=pkcs11
HSM_MODULE_PATH=/usr/lib/softhsm/libsofthsm2.so   # path to PKCS#11 .so / .dylib
HSM_TOKEN_LABEL=dev-signing-token                  # which token (partition) to use
HSM_PIN=1234                                       # user PIN to authenticate
HSM_KEY_LABEL=pdf-sign-key                         # which private key to use for signing
# HSM_KEY_ID=01                                    # hex CKA_ID — set if multiple keys share the same label
```

**Understanding the HSM env vars:**

| Var | Concept | Analogy |
|---|---|---|
| `HSM_TOKEN_LABEL` | Named partition/slot inside the HSM | A labelled drawer in the safe |
| `HSM_PIN` | User password to unlock the token | PIN to open that drawer |
| `HSM_KEY_LABEL` | CKA_LABEL of the private key object | Name of the specific key inside the drawer |
| `HSM_KEY_ID` | CKA_ID (hex) of the key — optional disambiguator | Key serial number; use when multiple keys share the same label |

**Module path by provider:**

| Provider | Module path |
|---|---|
| SoftHSM2 (Linux) | `/usr/lib/softhsm/libsofthsm2.so` |
| SoftHSM2 (macOS Homebrew) | `/opt/homebrew/lib/softhsm/libsofthsm2.dylib` |
| AWS CloudHSM | `/opt/cloudhsm/lib/libcloudhsm_pkcs11.so` |

**SoftHSM2 setup for local dev:**

```bash
# Install
brew install softhsm          # macOS
apt install softhsm2          # Ubuntu/Debian

# Initialize a token
softhsm2-util --init-token --free \
  --label "dev-signing-token" \
  --pin 1234 \
  --so-pin 0000

# Generate RSA-2048 key pair inside SoftHSM
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keypairgen --key-type rsa:2048 \
  --label "pdf-sign-key" \
  --id 01 \
  --usage-sign
```

> **Important:** When migrating from `file` to a real HSM, the private key inside the HSM must correspond to the public certificate embedded in the PDF's CMS structure by `msign-backend`.

**Why CKM_RSA_PKCS and not CKM_SHA256_RSA_PKCS?**

`CKM_SHA256_RSA_PKCS` hashes its input before signing — but this service receives a **pre-computed** digest. Using it would double-hash the data, producing an invalid PDF signature. The service uses `CKM_RSA_PKCS` (raw RSA) and manually prepends the ASN.1 DigestInfo prefix to match the output of Go's `rsa.SignPKCS1v15`.

**Production note — session pool:**

The current implementation uses a single PKCS#11 session protected by a mutex. This serializes all signing requests, which is fine for dev and low-traffic use. For high-throughput production, replace the mutex with a session pool (one session per goroutine up to a configurable max).

---

## Cryptographic Rules

These rules must be followed for the signature to be accepted by PDF validators (Adobe Acrobat, iText, etc.):

| Rule | Detail |
|---|---|
| **Do NOT re-hash** | The caller sends pre-hashed bytes. The service signs them raw — no SHA-256 inside. |
| **Padding: PKCS#1 v1.5** | Uses `rsa.SignPKCS1v15`. PSS padding produces an invalid PDF signature. |
| **Base64: Standard encoding** | Uses `base64.StdEncoding`. URL-safe or raw (no-padding) variants are not compatible. |
| **Key pair must match** | The private key used here must correspond to the public certificate embedded in the PDF's CMS structure by the caller. |

---

## Project Structure

```
hash-signing-service/
├── certs/                          # Certificate files (PEM)
│   ├── signing.key                 # RSA private key
│   ├── signing.crt                 # Signing certificate
│   ├── sub-ca.crt                  # Subordinate CA certificate
│   └── root-ca.crt                 # Root CA certificate
├── config/
│   └── config.go                   # App configuration, env loading
├── interfaces/
│   ├── handlers/
│   │   ├── hash_sign_handler.go    # POST /api/v1/hash-sign handler
│   │   └── root_handler.go         # GET / handler
│   ├── middleware/
│   │   ├── config_middleware.go    # Injects config into request context
│   │   └── logger_middleware.go    # Request/response logging
│   ├── routes/
│   │   └── route.go                # Route registration
│   └── services/
│       ├── signer.go               # Signer interface + OID maps + DigestInfo prefixes
│       ├── file_signer.go          # FileSigner — PEM key (SIGNER_BACKEND=file)
│       ├── hsm_signer.go           # HSMSigner — PKCS#11 (SIGNER_BACKEND=pkcs11)
│       ├── hash_sign_service.go    # Validates request, delegates to Signer
│       └── cert_services.go        # Certificate and key loading
├── pkg/
│   ├── requests/
│   │   └── request_hash_sign.go    # Request struct
│   └── responses/
│       ├── response_global.go      # Shared response helpers
│       └── response_hash_sign.go   # Sign response structs
├── .env.example
├── go.mod
├── go.sum
└── main.go                         # Entry point, cert/key loading, server start
```

---

## Testing

**Run all tests**

```bash
go test ./...
```

**Run with verbose output**

```bash
go test ./... -v
```

**Run specific package**

```bash
# Service layer only
go test ./interfaces/services/... -v

# Handler layer only
go test ./interfaces/handlers/... -v
```

**Run specific test**

```bash
go test ./interfaces/services/... -run TestCall_DoesNotReHash -v
go test ./interfaces/handlers/... -run TestHashSign_MultipleHashes -v
```

**Test coverage**

```bash
go test ./... -cover
```

**Coverage report in browser**

```bash
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

### Test coverage scope

| Package | Cases covered |
|---|---|
| `interfaces/services` | Valid signing (SHA-256, SHA-512), unsupported OID, invalid base64, different inputs produce different signatures, no re-hash guard |
| `interfaces/handlers` | 200 single hash, 200 multiple hashes in order, 400 missing params, 400 empty hash element, 400 unsupported OID, 400 invalid base64, 500 no config, 500 nil private key, Content-Type header |

---

## Verify Signature (optional)

```bash
HASH=$(echo -n "signed attributes content" | openssl dgst -sha256 -binary | base64)

SIG=$(curl -sS -X POST http://localhost:7777/api/v1/hash-sign \
  -H 'Content-Type: application/json' \
  -d "{\"hash\":[\"$HASH\"],\"hash_algo\":\"2.16.840.1.101.3.4.2.1\",\"sign_algo\":\"1.2.840.113549.1.1.11\"}" \
  | jq -r '.signatures[0]')

echo "$SIG" | base64 -d > sig.bin
echo -n "signed attributes content" | openssl dgst -sha256 -binary > hash.bin

# Decrypted signature bytes should match the digest bytes
openssl rsautl -verify -inkey certs/signing.key -in sig.bin -raw | xxd
xxd hash.bin
```
