package security

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTValidator chiu trach nhiem xac thuc JWT token va chiet xuat claims.
type JWTValidator struct {
	secretKey []byte
	issuer    string
	audience  string
}

// NewJWTValidator khoi tao mot JWTValidator. Khoi tao tu bien moi truong JWT_SECRET.
func NewJWTValidator() *JWTValidator {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-policy-engine-super-secret-key-12345"
	}
	return NewJWTValidatorWithConfig(secret, os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE"))
}

// NewJWTValidatorWithConfig builds a validator from already-loaded application
// configuration. Non-empty issuer and audience are enforced during parsing.
func NewJWTValidatorWithConfig(secret, issuer, audience string) *JWTValidator {
	return &JWTValidator{secretKey: []byte(secret), issuer: issuer, audience: audience}
}

// ValidateToken xac thuc chu ky va thoi gian song cua token, tra ve map cac claims.
func (v *JWTValidator) ValidateToken(tokenStr string) (jwt.MapClaims, error) {
	// Bo di tien to Bearer neu co
	if strings.HasPrefix(tokenStr, "Bearer ") {
		tokenStr = tokenStr[7:]
	}

	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	}
	if v.issuer != "" {
		options = append(options, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		options = append(options, jwt.WithAudience(v.audience))
	}
	token, err := jwt.Parse(
		tokenStr,
		func(_ *jwt.Token) (interface{}, error) { return v.secretKey, nil },
		options...,
	)

	if err != nil {
		return nil, fmt.Errorf("loi parse token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("token khong hop le")
}

// ExtractSubjectAttributes chiet xuat identity tho (subject) va cac thuoc tinh ngu canh (attributes) trong claims.
func (v *JWTValidator) ExtractSubjectAttributes(claims jwt.MapClaims) (string, map[string]string, error) {
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", nil, fmt.Errorf("thieu hoac sai kieu truong 'sub' trong token claims")
	}

	attrs := make(map[string]string)
	for k, val := range claims {
		// Bỏ qua các trường metadata JWT mặc định
		if k == "sub" || k == "exp" || k == "iat" || k == "nbf" || k == "iss" || k == "aud" || k == "jti" {
			continue
		}

		switch v := val.(type) {
		case string:
			attrs[k] = v
		case bool:
			attrs[k] = fmt.Sprintf("%t", v)
		case float64:
			attrs[k] = fmt.Sprintf("%g", v)
		default:
			// Neu la kieu du lieu phuc tap (array/object), map sang JSON String
			// de Evaluator co the check bang cac phep so khop chuoi/ton tai
			// (Luu y: trong du an hien tai Evaluator chu yeu doc map[string]string)
			attrs[k] = fmt.Sprintf("%v", v)
		}
	}

	return sub, attrs, nil
}

// HasPermission checks an authorization permission carried by a signed JWT.
// It supports a JSON string array and comma/space-delimited strings so common
// identity providers can map their permission claim without custom decoding.
func HasPermission(claims jwt.MapClaims, required string) bool {
	value, exists := claims["permissions"]
	if !exists || required == "" {
		return false
	}

	switch permissions := value.(type) {
	case string:
		for _, permission := range strings.FieldsFunc(permissions, func(r rune) bool {
			return r == ',' || r == ' '
		}) {
			if permission == required {
				return true
			}
		}
	case []string:
		for _, permission := range permissions {
			if permission == required {
				return true
			}
		}
	case []interface{}:
		for _, permission := range permissions {
			if text, ok := permission.(string); ok && text == required {
				return true
			}
		}
	}

	return false
}
