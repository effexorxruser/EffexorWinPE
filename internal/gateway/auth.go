package gateway

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxTokenHashFileBytes = 64 << 10

// TokenVerifier keeps only SHA-256 digests of revocable device tokens. The raw
// token is stored on removable technician media and never belongs in the image.
type TokenVerifier struct {
	hashes [][sha256.Size]byte
}

func LoadTokenVerifier(path string) (TokenVerifier, error) {
	file, err := os.Open(path)
	if err != nil {
		return TokenVerifier{}, err
	}
	defer file.Close()
	return ParseTokenVerifier(io.LimitReader(file, maxTokenHashFileBytes+1))
}

func ParseTokenVerifier(reader io.Reader) (TokenVerifier, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return TokenVerifier{}, err
	}
	if len(data) > maxTokenHashFileBytes {
		return TokenVerifier{}, fmt.Errorf("device-token hash file exceeds %d bytes", maxTokenHashFileBytes)
	}

	verifier := TokenVerifier{}
	seen := map[[sha256.Size]byte]struct{}{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) != sha256.Size*2 {
			return TokenVerifier{}, fmt.Errorf("device-token digest must contain 64 hexadecimal characters")
		}
		decoded, err := hex.DecodeString(line)
		if err != nil {
			return TokenVerifier{}, fmt.Errorf("decode device-token digest: %w", err)
		}
		var digest [sha256.Size]byte
		copy(digest[:], decoded)
		if _, exists := seen[digest]; exists {
			continue
		}
		seen[digest] = struct{}{}
		verifier.hashes = append(verifier.hashes, digest)
	}
	if err := scanner.Err(); err != nil {
		return TokenVerifier{}, err
	}
	if len(verifier.hashes) == 0 {
		return TokenVerifier{}, fmt.Errorf("device-token hash file contains no credentials")
	}
	return verifier, nil
}

// VerifyAuthorization returns a stable non-secret principal identifier. Every
// configured digest is compared so token position is not leaked by early exit.
func (v TokenVerifier) VerifyAuthorization(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := parts[1]
	if token == "" || len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		return "", false
	}
	digest := sha256.Sum256([]byte(token))
	matched := 0
	for _, expected := range v.hashes {
		matched |= subtle.ConstantTimeCompare(digest[:], expected[:])
	}
	if matched != 1 {
		return "", false
	}
	return hex.EncodeToString(digest[:]), true
}
