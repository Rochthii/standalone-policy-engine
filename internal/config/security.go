package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type SecurityConfig struct {
	JWTSecret                 string
	JWTIssuer                 string
	JWTAudience               string
	DelegationSecret          string // Development compatibility fallback only.
	DelegationActiveKeyID     string
	DelegationKeys            map[string]string
	DelegationKeyringExplicit bool
	ApprovalActiveKeyID       string
	ApprovalKeys              map[string]string
	ApprovalKeyringExplicit   bool
	ApprovalTTL               time.Duration
	TLSCertFile               string
	TLSKeyFile                string
	TLSCAFile                 string
	AuditActiveKeyID          string
	AuditKeys                 map[string]string
	AuditKeyringExplicit      bool
}

const (
	developmentJWTSecret        = "standalone-policy-engine-dev-jwt-secret"
	developmentDelegationSecret = "standalone-policy-engine-dev-delegation-secret"
	developmentApprovalSecret   = "standalone-policy-engine-dev-approval-secret"
	developmentAuditKEK         = "development-audit-kek-32-bytes!!"
)

func validateProductionConfig(cfg *Config) error {
	if !strings.EqualFold(cfg.AppEnv, "production") {
		return nil
	}
	if cfg.Database.URL == "" || strings.Contains(strings.ToLower(cfg.Database.URL), "localhost") {
		return errors.New("DATABASE_URL tren Production khong duoc de trong hoac tro vao localhost")
	}
	if len(cfg.Security.JWTSecret) < 32 || cfg.Security.JWTSecret == developmentJWTSecret {
		return errors.New("JWT_SECRET tren Production phai la secret rieng, toi thieu 32 ky tu")
	}
	if strings.TrimSpace(cfg.Security.JWTIssuer) == "" || cfg.Security.JWTIssuer == "standalone-policy-engine-dev" {
		return errors.New("JWT_ISSUER tren Production phai duoc cau hinh ro rang")
	}
	if strings.TrimSpace(cfg.Security.JWTAudience) == "" || cfg.Security.JWTAudience == "standalone-policy-engine-pdp" {
		return errors.New("JWT_AUDIENCE tren Production phai duoc cau hinh ro rang")
	}
	if err := validateProductionDelegationKeyring(cfg.Security); err != nil {
		return err
	}
	if !cfg.Security.ApprovalKeyringExplicit {
		return errors.New("PDP_APPROVAL_KEYS_JSON va PDP_APPROVAL_ACTIVE_KID bat buoc tren Production")
	}
	if err := validateApprovalKeyring(cfg.Security.ApprovalActiveKeyID, cfg.Security.ApprovalKeys); err != nil {
		return err
	}
	if !cfg.Security.AuditKeyringExplicit {
		return errors.New("LOG_KEKS_JSON va LOG_KEK_ACTIVE_KID bat buoc tren Production")
	}
	if err := validateAuditKeyring(cfg.Security.AuditActiveKeyID, cfg.Security.AuditKeys); err != nil {
		return err
	}
	if cfg.Audit.ArchiveBucket == "" {
		return errors.New("AUDIT_ARCHIVE_BUCKET bat buoc tren Production")
	}
	if strings.TrimSpace(cfg.Security.TLSCertFile) == "" || strings.TrimSpace(cfg.Security.TLSKeyFile) == "" || strings.TrimSpace(cfg.Security.TLSCAFile) == "" {
		return errors.New("PDP_TLS_CERT, PDP_TLS_KEY va PDP_TLS_CA bat buoc tren Production")
	}
	return nil
}

func loadAuditKeyring() (string, map[string]string, bool, error) {
	activeKeyID, activeExplicit := os.LookupEnv("LOG_KEK_ACTIVE_KID")
	rawKeys, keysExplicit := os.LookupEnv("LOG_KEKS_JSON")
	if !activeExplicit && !keysExplicit {
		legacyKey, legacyExplicit := os.LookupEnv("LOG_KEK")
		if !legacyExplicit || legacyKey == "" {
			legacyKey = developmentAuditKEK
		}
		if len(legacyKey) != 32 {
			return "", nil, false, errors.New("LOG_KEK phai dai chinh xac 32 bytes")
		}
		return "legacy", map[string]string{"legacy": legacyKey}, false, nil
	}
	if strings.TrimSpace(activeKeyID) == "" || strings.TrimSpace(rawKeys) == "" {
		return "", nil, false, errors.New("LOG_KEK_ACTIVE_KID va LOG_KEKS_JSON phai duoc cau hinh cung nhau")
	}

	keys := make(map[string]string)
	if err := json.Unmarshal([]byte(rawKeys), &keys); err != nil {
		return "", nil, false, fmt.Errorf("LOG_KEKS_JSON khong hop le: %w", err)
	}
	if err := validateAuditKeyring(activeKeyID, keys); err != nil {
		return "", nil, false, err
	}
	return activeKeyID, keys, true, nil
}

func validateAuditKeyring(activeKeyID string, keys map[string]string) error {
	if strings.TrimSpace(activeKeyID) == "" {
		return errors.New("audit active key id khong duoc rong")
	}
	if _, exists := keys[activeKeyID]; !exists {
		return errors.New("LOG_KEK_ACTIVE_KID khong co trong key ring")
	}
	for keyID, key := range keys {
		if strings.TrimSpace(keyID) == "" || len(key) != 32 {
			return fmt.Errorf("audit KEK %q phai dai chinh xac 32 bytes", keyID)
		}
	}
	return nil
}

func validateProductionDelegationKeyring(cfg SecurityConfig) error {
	if !cfg.DelegationKeyringExplicit {
		return errors.New("PDP_DELEGATION_KEYS_JSON va PDP_DELEGATION_ACTIVE_KID bat buoc tren Production")
	}
	activeSecret, exists := cfg.DelegationKeys[cfg.DelegationActiveKeyID]
	if !exists || len(activeSecret) < 32 {
		return errors.New("delegation active key tren Production phai ton tai va dai toi thieu 32 ky tu")
	}
	for keyID, secret := range cfg.DelegationKeys {
		if len(secret) < 32 {
			return fmt.Errorf("delegation key %q tren Production phai dai toi thieu 32 ky tu", keyID)
		}
	}
	return nil
}

func loadDelegationKeyring(fallbackSecret string) (string, map[string]string, bool, error) {
	activeKeyID, activeExplicit := os.LookupEnv("PDP_DELEGATION_ACTIVE_KID")
	rawKeys, keysExplicit := os.LookupEnv("PDP_DELEGATION_KEYS_JSON")
	if !activeExplicit && !keysExplicit {
		return "legacy", map[string]string{"legacy": fallbackSecret}, false, nil
	}
	if strings.TrimSpace(activeKeyID) == "" || strings.TrimSpace(rawKeys) == "" {
		return "", nil, false, errors.New("PDP_DELEGATION_ACTIVE_KID va PDP_DELEGATION_KEYS_JSON phai duoc cau hinh cung nhau")
	}

	keys := make(map[string]string)
	if err := json.Unmarshal([]byte(rawKeys), &keys); err != nil {
		return "", nil, false, fmt.Errorf("PDP_DELEGATION_KEYS_JSON khong hop le: %w", err)
	}
	if len(keys) == 0 {
		return "", nil, false, errors.New("PDP_DELEGATION_KEYS_JSON khong duoc rong")
	}
	if _, exists := keys[activeKeyID]; !exists {
		return "", nil, false, errors.New("PDP_DELEGATION_ACTIVE_KID khong co trong key ring")
	}
	return activeKeyID, keys, true, nil
}

func loadApprovalKeyring() (string, map[string]string, bool, error) {
	activeKeyID, activeExplicit := os.LookupEnv("PDP_APPROVAL_ACTIVE_KID")
	rawKeys, keysExplicit := os.LookupEnv("PDP_APPROVAL_KEYS_JSON")
	if !activeExplicit && !keysExplicit {
		return "approval-legacy", map[string]string{"approval-legacy": developmentApprovalSecret}, false, nil
	}
	if strings.TrimSpace(activeKeyID) == "" || strings.TrimSpace(rawKeys) == "" {
		return "", nil, false, errors.New("PDP_APPROVAL_ACTIVE_KID va PDP_APPROVAL_KEYS_JSON phai duoc cau hinh cung nhau")
	}
	keys := make(map[string]string)
	if err := json.Unmarshal([]byte(rawKeys), &keys); err != nil {
		return "", nil, false, fmt.Errorf("PDP_APPROVAL_KEYS_JSON khong hop le: %w", err)
	}
	if err := validateApprovalKeyring(activeKeyID, keys); err != nil {
		return "", nil, false, err
	}
	return activeKeyID, keys, true, nil
}

func validateApprovalKeyring(activeKeyID string, keys map[string]string) error {
	if strings.TrimSpace(activeKeyID) == "" {
		return errors.New("approval active key id khong duoc rong")
	}
	if _, exists := keys[activeKeyID]; !exists {
		return errors.New("PDP_APPROVAL_ACTIVE_KID khong co trong key ring")
	}
	for keyID, key := range keys {
		if strings.TrimSpace(keyID) == "" || len(key) < 32 {
			return fmt.Errorf("approval key %q phai dai toi thieu 32 bytes", keyID)
		}
	}
	return nil
}

func validateSecurityKeySeparation(cfg SecurityConfig) error {
	used := make(map[string]string)
	register := func(domain, secret string) error {
		if secret == "" {
			return nil
		}
		if previous, exists := used[secret]; exists {
			return fmt.Errorf("security key material reused across %s and %s", previous, domain)
		}
		used[secret] = domain
		return nil
	}
	if err := register("jwt", cfg.JWTSecret); err != nil {
		return err
	}
	for _, entry := range []struct {
		domain string
		keys   map[string]string
	}{
		{domain: "delegation", keys: cfg.DelegationKeys},
		{domain: "approval", keys: cfg.ApprovalKeys},
		{domain: "audit", keys: cfg.AuditKeys},
	} {
		for _, secret := range entry.keys {
			if err := register(entry.domain, secret); err != nil {
				return err
			}
		}
	}
	return nil
}
