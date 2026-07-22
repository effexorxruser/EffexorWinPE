package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestTokenVerifierAcceptsConfiguredDigestWithoutStoringRawToken(t *testing.T) {
	digest := sha256.Sum256([]byte("device-secret"))
	verifier, err := ParseTokenVerifier(strings.NewReader("# technician token\n" + hex.EncodeToString(digest[:]) + "\n"))
	if err != nil {
		t.Fatalf("ParseTokenVerifier() error = %v", err)
	}
	principal, ok := verifier.VerifyAuthorization("Bearer device-secret")
	if !ok || principal != hex.EncodeToString(digest[:]) {
		t.Fatalf("VerifyAuthorization() = %q, %v", principal, ok)
	}
	if _, ok := verifier.VerifyAuthorization("Bearer wrong-secret"); ok {
		t.Fatal("VerifyAuthorization() accepted an unknown token")
	}
}

func TestTokenVerifierRejectsMalformedOrEmptyHashFiles(t *testing.T) {
	for _, contents := range []string{"", "# comments only\n", "not-a-sha256\n"} {
		if _, err := ParseTokenVerifier(strings.NewReader(contents)); err == nil {
			t.Fatalf("ParseTokenVerifier(%q) error = nil", contents)
		}
	}
}
