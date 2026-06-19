# MesignSigning — Hash Signing Microservice: Language-Agnostic Implementation Guide

## 1. Overview

`MesignSigning` is a stateless hash-signing microservice. It exposes a single endpoint
that receives a **pre-computed base64 digest**, signs it with an RSA private key, and
returns the base64 signature. No PDF handling. No database. No session.

The caller (`msign-backend`) builds the CMS structure, embeds the cert, and constructs
the final PAdES signature. The service only does the RSA operation.

---

## 2. Endpoint Contract

```
POST /api/v1/hash-sign
Content-Type: application/json
(NO AUTH — lock down at network layer)

Request:
{
  "hash":      ["<base64 pre-computed digest>"],   // array, always 1 element
  "hash_algo": "<OID>",                            // digest OID (see §3)
  "sign_algo": "<OID>"                             // signing OID (see §3)
}

200 OK:
{
  "signatures": ["<base64 RSA signature>"]
}

Error:
{
  "error": { "code": "<machine_code>", "message": "<human msg>" }
}
```

### Error → HTTP mapping

| Cause                        | HTTP | `error.code`     |
|------------------------------|------|------------------|
| Missing/blank required param | 400  | `bad_request`    |
| Unsupported `hash_algo` OID  | 400  | `bad_request`    |
| Any other error              | 500  | `internal_error` |

---

## 3. Supported OIDs

| Field       | OID                       | Algorithm               |
|-------------|---------------------------|-------------------------|
| `hash_algo` | `2.16.840.1.101.3.4.2.1`  | SHA-256                 |
| `hash_algo` | `2.16.840.1.101.3.4.2.2`  | SHA-384                 |
| `hash_algo` | `2.16.840.1.101.3.4.2.3`  | SHA-512                 |
| `sign_algo` | `1.2.840.113549.1.1.11`   | sha256WithRSAEncryption |

---

## 4. Cryptographic Rules (MUST follow for valid PDF signatures)

These rules apply regardless of implementation language.

| Rule | Detail |
|------|--------|
| **Do NOT re-hash** | The caller sends pre-hashed bytes. Sign them raw — do not run SHA-256 again inside the service. |
| **Padding: PKCS#1 v1.5** | Use `RSA_PKCS1_PADDING` / `PKCS1v15`. Do NOT use PSS — different padding = invalid PDF. |
| **Key pair must match** | The private key in this service must correspond to the public certificate embedded by the caller in the PDF's CMS structure. |
| **Return raw RSA output** | Return the raw RSA signature bytes, base64-encoded. Do not wrap in ASN.1 / DER again. |
| **One hash per request** | `hash` array always has 1 element (`numSignatures = 1`). |

---

## 5. Reference Implementation — Ruby

```ruby
# Signing core (OpenSSL Ruby 3.x — sign_raw = raw RSA PKCS#1 v1.5, no re-hash)
HASH_ALGO_MAP = {
  '2.16.840.1.101.3.4.2.1' => 'SHA256',
  '2.16.840.1.101.3.4.2.2' => 'SHA384',
  '2.16.840.1.101.3.4.2.3' => 'SHA512'
}.freeze

def sign(hash_b64)
  hash_bytes = Base64.strict_decode64(hash_b64)
  sig_bytes  = private_key.sign_raw(digest_name, hash_bytes)
  Base64.strict_encode64(sig_bytes)
end
```

---

## 6. Go Implementation

### 6.1 Key points

- Use `rsa.SignPKCS1v15(rand.Reader, key, hashFunc, hashBytes)` where `hashBytes` is
  the decoded digest — Go does NOT re-hash when you pass the bytes directly.
- `crypto.Hash` value tells Go which DigestInfo OID to embed in the PKCS#1 signature.
- Standard lib only: `crypto/rsa`, `crypto/x509`, `encoding/pem`, `encoding/base64`.

### 6.2 Full implementation

```go
package main

import (
    "crypto"
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "errors"
    "net/http"
    "os"
)

var hashOIDMap = map[string]crypto.Hash{
    "2.16.840.1.101.3.4.2.1": crypto.SHA256,
    "2.16.840.1.101.3.4.2.2": crypto.SHA384,
    "2.16.840.1.101.3.4.2.3": crypto.SHA512,
}

type signRequest struct {
    Hash     []string `json:"hash"`
    HashAlgo string   `json:"hash_algo"`
    SignAlgo string   `json:"sign_algo"`
}

type signResponse struct {
    Signatures []string `json:"signatures"`
}

type errorBody struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
}

func handleHashSign(privateKey *rsa.PrivateKey) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }

        var req signRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Hash) == 0 || req.HashAlgo == "" {
            renderError(w, http.StatusBadRequest, "bad_request", "missing required params")
            return
        }

        hashFunc, ok := hashOIDMap[req.HashAlgo]
        if !ok {
            renderError(w, http.StatusBadRequest, "bad_request", "unsupported hash_algo: "+req.HashAlgo)
            return
        }

        hashBytes, err := base64.StdEncoding.DecodeString(req.Hash[0])
        if err != nil {
            renderError(w, http.StatusBadRequest, "bad_request", "invalid base64 hash")
            return
        }

        // PKCS#1 v1.5, pre-hashed — hashBytes are the digest, NOT the original content
        sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, hashFunc, hashBytes)
        if err != nil {
            renderError(w, http.StatusInternalServerError, "internal_error", "signing failed")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(signResponse{
            Signatures: []string{base64.StdEncoding.EncodeToString(sigBytes)},
        })
    }
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    block, _ := pem.Decode(data)
    if block == nil {
        return nil, errors.New("failed to decode PEM block")
    }
    // Try PKCS#8 first, fallback to PKCS#1
    key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
    if err != nil {
        return x509.ParsePKCS1PrivateKey(block.Bytes)
    }
    rsaKey, ok := key.(*rsa.PrivateKey)
    if !ok {
        return nil, errors.New("PEM does not contain an RSA key")
    }
    return rsaKey, nil
}

func renderError(w http.ResponseWriter, status int, code, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    var body errorBody
    body.Error.Code = code
    body.Error.Message = msg
    json.NewEncoder(w).Encode(body)
}

func main() {
    keyPath := os.Getenv("MSIGN_PRIVATE_KEY_PATH")
    if keyPath == "" {
        panic("MSIGN_PRIVATE_KEY_PATH is required")
    }

    privateKey, err := loadPrivateKey(keyPath)
    if err != nil {
        panic("cannot load private key: " + err.Error())
    }

    http.HandleFunc("/api/v1/hash-sign", handleHashSign(privateKey))

    port := os.Getenv("PORT")
    if port == "" {
        port = "3001"
    }
    http.ListenAndServe(":"+port, nil)
}
```

### 6.3 ENV vars

```dotenv
MSIGN_PRIVATE_KEY_PATH=/path/to/private_key.pem   # PKCS#8 or PKCS#1 PEM
PORT=3001
```

### 6.4 Run

```bash
go run main.go
# or
go build -o msign-signing && ./msign-signing
```

### 6.5 Base64 variant — IMPORTANT

| | Encoding |
|-|----------|
| Ruby `Base64.strict_encode64` | Standard alphabet, no newlines |
| Go `base64.StdEncoding` | Standard alphabet, no newlines ✓ compatible |
| Go `base64.URLEncoding` | URL-safe alphabet — **do NOT use** |
| Go `base64.RawStdEncoding` | No padding — **do NOT use** |

---

## 7. Other Languages — Core Snippet

### Python

```python
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.asymmetric.utils import Prehashed
import base64

HASH_ALGO_MAP = {
    "2.16.840.1.101.3.4.2.1": hashes.SHA256(),
    "2.16.840.1.101.3.4.2.2": hashes.SHA384(),
    "2.16.840.1.101.3.4.2.3": hashes.SHA512(),
}

def sign_hash(hash_b64: str, hash_algo_oid: str, private_key) -> str:
    hash_bytes = base64.b64decode(hash_b64)
    algo       = HASH_ALGO_MAP[hash_algo_oid]
    # Prehashed tells cryptography lib: do NOT hash again
    signature  = private_key.sign(hash_bytes, padding.PKCS1v15(), Prehashed(algo))
    return base64.b64encode(signature).decode()
```

### Node.js

```js
const crypto = require('crypto')

const HASH_ALGO_MAP = {
  '2.16.840.1.101.3.4.2.1': 'SHA256',
  '2.16.840.1.101.3.4.2.2': 'SHA384',
  '2.16.840.1.101.3.4.2.3': 'SHA512',
}

function signHash(hashB64, hashAlgoOid, privateKeyPem) {
  const hashBytes = Buffer.from(hashB64, 'base64')
  const algo      = HASH_ALGO_MAP[hashAlgoOid]
  const sig = crypto.sign(algo, hashBytes, {
    key:     privateKeyPem,
    padding: crypto.constants.RSA_PKCS1_PADDING,
  })
  return sig.toString('base64')
}
```

---

## 8. Smoke Test (curl)

Works against any implementation:

```bash
# Simulate a SHA-256 hash (32 bytes) as HexaPDF would produce
HASH=$(echo -n "signed attributes content" | openssl dgst -sha256 -binary | base64)

curl -sS -X POST http://localhost:3001/api/v1/hash-sign \
  -H 'Content-Type: application/json' \
  -d "{
    \"hash\":      [\"$HASH\"],
    \"hash_algo\": \"2.16.840.1.101.3.4.2.1\",
    \"sign_algo\":  \"1.2.840.113549.1.1.11\"
  }" | jq .

# Expected:
# { "signatures": ["<base64 RSA signature>"] }
```

### Verify signature against public key

```bash
SIG=$(curl -sS -X POST http://localhost:3001/api/v1/hash-sign \
  -H 'Content-Type: application/json' \
  -d "{\"hash\":[\"$HASH\"],\"hash_algo\":\"2.16.840.1.101.3.4.2.1\",\"sign_algo\":\"1.2.840.113549.1.1.11\"}" \
  | jq -r '.signatures[0]')

echo "$SIG" | base64 -d > sig.bin
echo -n "signed attributes content" | openssl dgst -sha256 -binary > hash.bin

# Verify: decrypted signature should equal the digest
openssl rsautl -verify -inkey public.pem -pubin -in sig.bin -raw | xxd
xxd hash.bin
# bytes must match
```

---

## 9. Interoperability Checklist

Before swapping implementations, verify:

- [ ] Same private key PEM used (key pair matches the public cert embedded in PDF)
- [ ] Padding: PKCS#1 v1.5 — NOT PSS
- [ ] Hash: NOT re-computed inside the service (pass-through pre-hashed bytes)
- [ ] Base64: standard encoding, no URL-safe variant, no line breaks
- [ ] Response key is `"signatures"` (array), not `"signature"` (string)
- [ ] HTTP 200 on success, JSON error body on 4xx/5xx
- [ ] End-to-end: open signed PDF in Adobe Acrobat → Signatures panel → validate

---

## 10. Why Any Language Works

The PDF validator (Adobe, iText, etc.) only checks:

1. The bytes in the PDF `ByteRange` hash to the expected digest
2. The RSA signature over those bytes verifies against the public cert in the CMS
3. The cert chain is valid (or at least self-consistent for a demo cert)

It knows nothing about what language produced the signature. As long as the math is
correct — right key, right padding, right digest — the signature is valid.
