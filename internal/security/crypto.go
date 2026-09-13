package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const developmentAuditKEK = "development-audit-kek-32-bytes!!"

// EnvelopeCrypto encrypts audit payloads with per-record DEKs and a rotatable KEK ring.
type EnvelopeCrypto struct {
	activeKeyID string
	keks        map[string][]byte
}

// NewEnvelopeCrypto retains the single-key development API.
func NewEnvelopeCrypto() (*EnvelopeCrypto, error) {
	key := os.Getenv("LOG_KEK")
	if key == "" {
		key = developmentAuditKEK
	}
	return NewEnvelopeCryptoWithKeyring("legacy", map[string]string{"legacy": key})
}

func NewEnvelopeCryptoWithKeyring(activeKeyID string, keys map[string]string) (*EnvelopeCrypto, error) {
	if activeKeyID == "" {
		return nil, errors.New("audit active KEK id is required")
	}
	if _, ok := keys[activeKeyID]; !ok {
		return nil, fmt.Errorf("audit active KEK %q is absent from key ring", activeKeyID)
	}

	decoded := make(map[string][]byte, len(keys))
	for keyID, key := range keys {
		if keyID == "" || len(key) != 32 {
			return nil, fmt.Errorf("audit KEK %q must be exactly 32 bytes", keyID)
		}
		decoded[keyID] = []byte(key)
	}
	return &EnvelopeCrypto{activeKeyID: activeKeyID, keks: decoded}, nil
}

func (e *EnvelopeCrypto) ActiveKeyID() string { return e.activeKeyID }

// Encrypt retains the legacy API while using the active KEK.
func (e *EnvelopeCrypto) Encrypt(plaintext []byte) (string, string, error) {
	ciphertext, encryptedDEK, _, err := e.EncryptWithAAD(plaintext, nil)
	return ciphertext, encryptedDEK, err
}

func (e *EnvelopeCrypto) EncryptWithAAD(plaintext, aad []byte) (string, string, string, error) {
	dek := make([]byte, 32)
	defer clear(dek)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return "", "", "", fmt.Errorf("generate audit DEK: %w", err)
	}

	ciphertext, err := encryptAESGCM(plaintext, dek, aad)
	if err != nil {
		return "", "", "", fmt.Errorf("encrypt audit payload: %w", err)
	}
	encryptedDEK, err := encryptAESGCM(dek, e.keks[e.activeKeyID], []byte("audit-dek|v1|"+e.activeKeyID))
	if err != nil {
		return "", "", "", fmt.Errorf("wrap audit DEK: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(encryptedDEK), e.activeKeyID, nil
}

// Decrypt retains the legacy API and uses the active KEK.
func (e *EnvelopeCrypto) Decrypt(ciphertext, encryptedDEK string) ([]byte, error) {
	return e.DecryptWithAAD(ciphertext, encryptedDEK, e.activeKeyID, nil)
}

func (e *EnvelopeCrypto) DecryptWithAAD(ciphertextText, encryptedDEKText, keyID string, aad []byte) ([]byte, error) {
	kek, ok := e.keks[keyID]
	if !ok {
		return nil, fmt.Errorf("unknown audit KEK id %q", keyID)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextText)
	if err != nil {
		return nil, fmt.Errorf("decode audit ciphertext: %w", err)
	}
	encryptedDEK, err := base64.StdEncoding.DecodeString(encryptedDEKText)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted audit DEK: %w", err)
	}
	dek, err := decryptAESGCM(encryptedDEK, kek, []byte("audit-dek|v1|"+keyID))
	if err != nil {
		return nil, fmt.Errorf("unwrap audit DEK: %w", err)
	}
	defer clear(dek)
	plaintext, err := decryptAESGCM(ciphertext, dek, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt audit payload: %w", err)
	}
	return plaintext, nil
}

// IntegrityTag authenticates the complete stored envelope without exposing the KEK.
func (e *EnvelopeCrypto) IntegrityTag(keyID string, parts ...[]byte) (string, error) {
	kek, ok := e.keks[keyID]
	if !ok {
		return "", fmt.Errorf("unknown audit KEK id %q", keyID)
	}
	derived := hmac.New(sha256.New, kek)
	_, _ = derived.Write([]byte("audit-integrity-key|v1"))
	mac := hmac.New(sha256.New, derived.Sum(nil))
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		_, _ = mac.Write(size[:])
		_, _ = mac.Write(part)
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (e *EnvelopeCrypto) VerifyIntegrityTag(tag, keyID string, parts ...[]byte) error {
	want, err := e.IntegrityTag(keyID, parts...)
	if err != nil {
		return err
	}
	provided, err := base64.StdEncoding.DecodeString(tag)
	if err != nil {
		return errors.New("invalid audit integrity tag encoding")
	}
	expected, _ := base64.StdEncoding.DecodeString(want)
	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return errors.New("audit integrity verification failed")
	}
	return nil
}

func encryptAESGCM(plaintext, key, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, aad), nil
}

func decryptAESGCM(ciphertext, key, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aead.NonceSize() {
		return nil, errors.New("audit ciphertext is too short")
	}
	nonce := ciphertext[:aead.NonceSize()]
	return aead.Open(nil, nonce, ciphertext[aead.NonceSize():], aad)
}
