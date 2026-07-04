package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// EnvelopeCrypto chiu trach nhiem ma hoa va giai ma noi dung log kiem toan
// bang co che ma hoa phong bi (Envelope Encryption).
type EnvelopeCrypto struct {
	kek []byte // Key Encryption Key (Lay tu bien moi truong)
}

// NewEnvelopeCrypto khoi tao mot EnvelopeCrypto. Khoi tao tu bien moi truong LOG_KEK.
func NewEnvelopeCrypto() (*EnvelopeCrypto, error) {
	kekStr := os.Getenv("LOG_KEK")
	if kekStr == "" {
		// KEK mac dinh phai co do dai dung 32 bytes cho AES-256
		kekStr = "default-policy-engine-kek-key-32b"
	}
	
	kekBytes := []byte(kekStr)
	if len(kekBytes) != 32 {
		// Neu khong du 32 bytes thi pad hoac cat de dam bao an toan
		padded := make([]byte, 32)
		copy(padded, kekBytes)
		kekBytes = padded
	}

	return &EnvelopeCrypto{kek: kekBytes}, nil
}

// Encrypt thuc hien ma hoa du lieu plaintext su dung mot khoa DEK duoc sinh moi moi lan goi.
// Tra ve base64 ciphertext, base64 encryptedDEK va error.
func (e *EnvelopeCrypto) Encrypt(plaintext []byte) (string, string, error) {
	// 1. Sinh ngau nhien khoa DEK (32 bytes cho AES-256)
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return "", "", fmt.Errorf("loi sinh khoa DEK: %w", err)
	}

	// 2. Ma hoa noi dung plaintext bang khoa DEK su dung AES-GCM
	ciphertextBytes, err := encryptAESGCM(plaintext, dek)
	if err != nil {
		return "", "", fmt.Errorf("loi ma hoa du lieu bang DEK: %w", err)
	}

	// 3. Ma hoa khoa DEK bang khoa KEK su dung AES-GCM
	encryptedDEKBytes, err := encryptAESGCM(dek, e.kek)
	if err != nil {
		return "", "", fmt.Errorf("loi ma hoa DEK bang KEK: %w", err)
	}

	// 4. Encode sang base64 de luu database an toan
	ciphertext := base64.StdEncoding.EncodeToString(ciphertextBytes)
	encryptedDEK := base64.StdEncoding.EncodeToString(encryptedDEKBytes)

	return ciphertext, encryptedDEK, nil
}

// Decrypt thuc hien giai ma du lieu su dung base64 ciphertext va base64 encryptedDEK.
func (e *EnvelopeCrypto) Decrypt(ciphertextStr, encryptedDEKStr string) ([]byte, error) {
	// 1. Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextStr)
	if err != nil {
		return nil, fmt.Errorf("loi decode base64 ciphertext: %w", err)
	}

	encryptedDEK, err := base64.StdEncoding.DecodeString(encryptedDEKStr)
	if err != nil {
		return nil, fmt.Errorf("loi decode base64 encryptedDEK: %w", err)
	}

	// 2. Giai ma DEK bang khoa KEK
	dek, err := decryptAESGCM(encryptedDEK, e.kek)
	if err != nil {
		return nil, fmt.Errorf("loi giai ma DEK bang KEK: %w", err)
	}

	// 3. Giai ma ciphertext bang khoa DEK
	plaintext, err := decryptAESGCM(ciphertext, dek)
	if err != nil {
		return nil, fmt.Errorf("loi giai ma du lieu bang DEK: %w", err)
	}

	return plaintext, nil
}

// helper ma hoa AES-GCM
func encryptAESGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Sinh nonce ngau nhien
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Ma hoa du lieu va append nonce vao dau de sau nay trich xuat
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// helper giai ma AES-GCM
func decryptAESGCM(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext qua ngan")
	}

	// Trich xuat nonce va phan du lieu thuc te da ma hoa
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
