package security

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTValidateValidToken kiem thu xac thuc token hop le co chu ky dung.
func TestJWTValidateValidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-sprint-6-unit-test")
	defer os.Unsetenv("JWT_SECRET")

	v := NewJWTValidator()

	// Tao token hop le
	claims := jwt.MapClaims{
		"sub":        "user:alice",
		"department": "engineering",
		"role":       "admin",
		"exp":        time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))
	if err != nil {
		t.Fatalf("Khong the tao JWT token: %v", err)
	}

	parsedClaims, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken tra ve loi voi token hop le: %v", err)
	}
	if parsedClaims["sub"] != "user:alice" {
		t.Errorf("Truong 'sub' khong khop: mong %s, thuc te %v", "user:alice", parsedClaims["sub"])
	}
}

// TestJWTValidateBearerPrefix kiem thu loai bo tien to "Bearer ".
func TestJWTValidateBearerPrefix(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-sprint-6-unit-test")
	defer os.Unsetenv("JWT_SECRET")

	v := NewJWTValidator()

	claims := jwt.MapClaims{
		"sub": "user:bob",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))
	if err != nil {
		t.Fatalf("Khong the tao JWT token: %v", err)
	}

	// Gui kem tien to Bearer
	parsedClaims, err := v.ValidateToken("Bearer " + tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken voi tien to Bearer tra ve loi: %v", err)
	}
	if parsedClaims["sub"] != "user:bob" {
		t.Errorf("Sub khong khop: mong user:bob, thuc te %v", parsedClaims["sub"])
	}
}

// TestJWTValidateExpiredToken kiem thu tu choi token het han.
func TestJWTValidateExpiredToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-sprint-6-unit-test")
	defer os.Unsetenv("JWT_SECRET")

	v := NewJWTValidator()

	claims := jwt.MapClaims{
		"sub": "user:eve",
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // Da het han
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))
	if err != nil {
		t.Fatalf("Khong the tao JWT token: %v", err)
	}

	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("ValidateToken phai tra ve loi voi token da het han, nhung khong co loi")
	}
}

// TestJWTValidateWrongSecret kiem thu tu choi token ky bang secret sai.
func TestJWTValidateWrongSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "correct-secret-key-abcdef")
	defer os.Unsetenv("JWT_SECRET")

	v := NewJWTValidator()

	claims := jwt.MapClaims{
		"sub": "user:mallory",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	// Ky bang secret SAI
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("wrong-secret-key-xyz"))
	if err != nil {
		t.Fatalf("Khong the tao JWT token: %v", err)
	}

	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("ValidateToken phai tu choi token ky bang secret sai, nhung da chap nhan")
	}
}

// TestJWTExtractSubjectAttributes kiem thu chiet xuat sub va cac attributes phi JWT metadata.
func TestJWTExtractSubjectAttributes(t *testing.T) {
	v := NewJWTValidator()

	claims := jwt.MapClaims{
		"sub":        "user:carol",
		"department": "finance",
		"clearance":  "secret",
		"is_admin":   true,
		"exp":        time.Now().Add(1 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}

	sub, attrs, err := v.ExtractSubjectAttributes(claims)
	if err != nil {
		t.Fatalf("ExtractSubjectAttributes tra ve loi: %v", err)
	}
	if sub != "user:carol" {
		t.Errorf("Sub khong khop: mong user:carol, thuc te %s", sub)
	}

	// Cac truong exp, iat phai bi loc ra, khong duoc xuat hien trong attrs
	if _, exists := attrs["exp"]; exists {
		t.Error("Truong 'exp' khong duoc xuat hien trong subject attributes")
	}
	if _, exists := attrs["iat"]; exists {
		t.Error("Truong 'iat' khong duoc xuat hien trong subject attributes")
	}

	// Cac truong nghiep vu phai ton tai
	if attrs["department"] != "finance" {
		t.Errorf("Truong 'department' sai: mong finance, thuc te %s", attrs["department"])
	}
	if attrs["clearance"] != "secret" {
		t.Errorf("Truong 'clearance' sai: mong secret, thuc te %s", attrs["clearance"])
	}
}

// TestJWTExtractMissingSub kiem thu loi khi token thieu truong 'sub'.
func TestJWTExtractMissingSub(t *testing.T) {
	v := NewJWTValidator()

	claims := jwt.MapClaims{
		"department": "engineering",
		"exp":        time.Now().Add(1 * time.Hour).Unix(),
	}

	_, _, err := v.ExtractSubjectAttributes(claims)
	if err == nil {
		t.Error("ExtractSubjectAttributes phai tra ve loi khi thieu truong 'sub'")
	}
}
