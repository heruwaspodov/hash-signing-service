package services

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/miekg/pkcs11"
)

// HSMSigner signs via PKCS#11 using the CKM_RSA_PKCS mechanism.
// Compatible with SoftHSM2 (dev) and AWS CloudHSM.
// GCP Cloud KMS and Azure Key Vault use REST/SDK APIs, not a PKCS#11 module —
// those require a separate signer implementation.
// SIGNER_BACKEND=pkcs11.
type HSMSigner struct {
	ctx       *pkcs11.Ctx
	session   pkcs11.SessionHandle
	keyHandle pkcs11.ObjectHandle
	// mu serializes Sign calls — PKCS#11 sessions are not goroutine-safe.
	mu sync.Mutex
}

// NewHSMSigner initializes the PKCS#11 module, finds the token slot by label,
// opens a session, logs in, and locates the private key by label (and optionally by ID).
// keyID is a hex string (e.g. "01ab") matching CKA_ID — leave empty to match by label only.
// Returns a ready-to-use HSMSigner or an error if any step fails.
// All PKCS#11 resources are released if initialization fails.
func NewHSMSigner(modulePath, tokenLabel, pin, keyLabel, keyID string) (*HSMSigner, error) {
	ctx := pkcs11.New(modulePath)
	if err := ctx.Initialize(); err != nil {
		return nil, fmt.Errorf("pkcs11 initialize: %v", err)
	}

	// cleanup releases all PKCS#11 resources acquired so far.
	// Called on any error path after Initialize succeeds.
	cleanup := func(session *pkcs11.SessionHandle) {
		if session != nil {
			ctx.Logout(*session)
			ctx.CloseSession(*session)
		}
		ctx.Finalize()
		ctx.Destroy()
	}

	// Find the slot whose token label matches tokenLabel.
	slots, err := ctx.GetSlotList(true)
	if err != nil {
		cleanup(nil)
		return nil, fmt.Errorf("get slot list: %v", err)
	}

	var targetSlot uint
	found := false
	for _, slot := range slots {
		info, err := ctx.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		if strings.TrimRight(info.Label, " ") == tokenLabel {
			targetSlot = slot
			found = true
			break
		}
	}
	if !found {
		cleanup(nil)
		return nil, fmt.Errorf("token with label %q not found", tokenLabel)
	}

	// Open a read-write session required for signing.
	session, err := ctx.OpenSession(targetSlot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		cleanup(nil)
		return nil, fmt.Errorf("open session: %v", err)
	}

	// Authenticate as normal user.
	if err := ctx.Login(session, pkcs11.CKU_USER, pin); err != nil {
		cleanup(&session)
		return nil, fmt.Errorf("login: %v", err)
	}

	// Build search template — always filter by class and label.
	// Append CKA_ID if provided to disambiguate keys with duplicate labels.
	tmpl := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, keyLabel),
	}
	if keyID != "" {
		idBytes, err := hex.DecodeString(keyID)
		if err != nil {
			cleanup(&session)
			return nil, fmt.Errorf("invalid HSM_KEY_ID %q (must be hex): %v", keyID, err)
		}
		tmpl = append(tmpl, pkcs11.NewAttribute(pkcs11.CKA_ID, idBytes))
	}

	if err := ctx.FindObjectsInit(session, tmpl); err != nil {
		cleanup(&session)
		return nil, fmt.Errorf("find objects init: %v", err)
	}
	objects, _, err := ctx.FindObjects(session, 1)
	ctx.FindObjectsFinal(session)
	if err != nil {
		cleanup(&session)
		return nil, fmt.Errorf("find objects: %v", err)
	}
	if len(objects) == 0 {
		cleanup(&session)
		return nil, fmt.Errorf("private key with label %q (id: %q) not found in HSM", keyLabel, keyID)
	}

	return &HSMSigner{
		ctx:       ctx,
		session:   session,
		keyHandle: objects[0],
	}, nil
}

// Sign performs RSA PKCS#1 v1.5 signing via PKCS#11 CKM_RSA_PKCS mechanism.
//
// Unlike rsa.SignPKCS1v15 (which prepends DigestInfo automatically), CKM_RSA_PKCS
// performs a raw RSA operation. The ASN.1 DigestInfo prefix is prepended manually
// before calling C_Sign — required for the output to match rsa.SignPKCS1v15 and
// be accepted by PDF validators (Adobe, iText, etc.).
func (h *HSMSigner) Sign(hashBytes []byte, hashAlgoOID string) ([]byte, error) {
	prefix, ok := digestInfoPrefix[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	// Build DigestInfo || hash as required by CKM_RSA_PKCS.
	data := make([]byte, len(prefix)+len(hashBytes))
	copy(data, prefix)
	copy(data[len(prefix):], hashBytes)

	h.mu.Lock()
	defer h.mu.Unlock()

	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS, nil)}
	if err := h.ctx.SignInit(h.session, mech, h.keyHandle); err != nil {
		return nil, fmt.Errorf("sign init: %v", err)
	}

	sig, err := h.ctx.Sign(h.session, data)
	if err != nil {
		return nil, fmt.Errorf("sign: %v", err)
	}

	return sig, nil
}

// Close logs out and releases all PKCS#11 resources.
// Call via defer in main after the HTTP server stops.
func (h *HSMSigner) Close() {
	h.ctx.Logout(h.session)
	h.ctx.CloseSession(h.session)
	h.ctx.Finalize()
	h.ctx.Destroy()
}
