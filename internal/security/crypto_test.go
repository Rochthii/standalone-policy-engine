package security

import (
	"os"
	"strings"
	"testing"
)

// TestEnvelopeEncryptDecrypt kiem thu vong doi ma hoa va giai ma day du.
func TestEnvelopeEncryptDecrypt(t *testing.T) {
	os.Setenv("LOG_KEK", "test-kek-key-for-sprint6-32bytess")
	defer os.Unsetenv("LOG_KEK")

	crypto, err := NewEnvelopeCrypto()
	if err != nil {
		t.Fatalf("Khong the khoi tao EnvelopeCrypto: %v", err)
	}

	plaintext := []byte(`{"subject":"user:alice","action":"READ","resource":"file:report.pdf","context":{"ip":"10.0.0.1"}}`)

	ciphertext, encDEK, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt tra ve loi: %v", err)
	}

	if ciphertext == "" || encDEK == "" {
		t.Fatal("Ciphertext va encDEK khong duoc rong")
	}

	// Ciphertext phai khac plaintext
	if string(plaintext) == ciphertext {
		t.Error("Ciphertext phai khac plaintext, nhung chung giong nhau")
	}

	// Giai ma lai va kiem tra khop
	decrypted, err := crypto.Decrypt(ciphertext, encDEK)
	if err != nil {
		t.Fatalf("Decrypt tra ve loi: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Du lieu sau giai ma khong khop:\nMong: %s\nThuc te: %s", plaintext, decrypted)
	}
}

// TestEnvelopeUniqueNoncePerEncryption kiem thu moi lan ma hoa tao ra ciphertext khac nhau.
func TestEnvelopeUniqueNoncePerEncryption(t *testing.T) {
	os.Setenv("LOG_KEK", "test-kek-key-for-sprint6-32bytess")
	defer os.Unsetenv("LOG_KEK")

	crypto, err := NewEnvelopeCrypto()
	if err != nil {
		t.Fatalf("Khong the khoi tao EnvelopeCrypto: %v", err)
	}

	plaintext := []byte("same plaintext input for nonce test")

	ciphertext1, _, err1 := crypto.Encrypt(plaintext)
	ciphertext2, _, err2 := crypto.Encrypt(plaintext)

	if err1 != nil || err2 != nil {
		t.Fatalf("Encrypt loi: %v, %v", err1, err2)
	}

	// Hai lan ma hoa phai cho ra ciphertext khac nhau (do nonce ngau nhien)
	if ciphertext1 == ciphertext2 {
		t.Error("Hai ciphertext cua cung mot plaintext phai khac nhau do nonce ngau nhien, nhung chung giong nhau")
	}
}

// TestEnvelopeDecryptWithWrongDEK kiem thu giai ma that bai khi DEK sai.
func TestEnvelopeDecryptWithWrongDEK(t *testing.T) {
	os.Setenv("LOG_KEK", "test-kek-key-for-sprint6-32bytess")
	defer os.Unsetenv("LOG_KEK")

	crypto, err := NewEnvelopeCrypto()
	if err != nil {
		t.Fatalf("Khong the khoi tao EnvelopeCrypto: %v", err)
	}

	plaintext := []byte("sensitive audit log payload")
	ciphertext, _, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt loi: %v", err)
	}

	// Thu giai ma bang DEK gia mao (base64 hieu le)
	_, err = crypto.Decrypt(ciphertext, "aW52YWxpZC1kZWstZGF0YQ==")
	if err == nil {
		t.Error("Decrypt phai that bai khi su dung DEK sai, nhung lai thanh cong")
	}
}

// TestEnvelopeLargePayload kiem thu ma hoa payload lon (>1KB).
func TestEnvelopeLargePayload(t *testing.T) {
	os.Setenv("LOG_KEK", "test-kek-key-for-sprint6-32bytess")
	defer os.Unsetenv("LOG_KEK")

	crypto, err := NewEnvelopeCrypto()
	if err != nil {
		t.Fatalf("Khong the khoi tao EnvelopeCrypto: %v", err)
	}

	// Tao payload 5KB
	largePayload := []byte(strings.Repeat("audit-log-data-entry|", 240))

	ciphertext, encDEK, err := crypto.Encrypt(largePayload)
	if err != nil {
		t.Fatalf("Encrypt payload lon tra ve loi: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext, encDEK)
	if err != nil {
		t.Fatalf("Decrypt payload lon tra ve loi: %v", err)
	}

	if string(decrypted) != string(largePayload) {
		t.Error("Payload lon sau giai ma khong khop voi ban goc")
	}
}

// TestEnvelopeDefaultKEK kiem thu hoat dong binh thuong khi bien moi truong LOG_KEK khong dat.
func TestEnvelopeDefaultKEK(t *testing.T) {
	os.Unsetenv("LOG_KEK")

	crypto, err := NewEnvelopeCrypto()
	if err != nil {
		t.Fatalf("Khong the khoi tao EnvelopeCrypto voi KEK mac dinh: %v", err)
	}

	plaintext := []byte("test with default KEK")
	ciphertext, encDEK, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt voi KEK mac dinh loi: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext, encDEK)
	if err != nil {
		t.Fatalf("Decrypt voi KEK mac dinh loi: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Error("Du lieu giai ma voi KEK mac dinh khong khop")
	}
}
