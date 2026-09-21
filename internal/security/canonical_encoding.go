package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

func writeCanonicalString(payload *bytes.Buffer, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	payload.Write(length[:])
	payload.WriteString(value)
}

func writeCanonicalInt64(payload *bytes.Buffer, value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	payload.Write(encoded[:])
}

func writeCanonicalUint32(payload *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	payload.Write(encoded[:])
}

func decodeCanonicalDigest(field, value string) ([]byte, error) {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return nil, fmt.Errorf("%s must be lowercase SHA-256 hex", field)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be lowercase SHA-256 hex", field)
	}
	return decoded, nil
}

func validateCanonicalText(field, value string) error {
	if value == "" || uint64(len(value)) > uint64(math.MaxUint32) || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must be non-empty canonical UTF-8", field)
	}
	return nil
}

func validateCanonicalSubject(field, value, prefix string) error {
	if err := validateCanonicalText(field, value); err != nil {
		return err
	}
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return fmt.Errorf("%s must use %s subject prefix", field, prefix)
	}
	return nil
}

func validateCanonicalTimestamp(field, value string) error {
	const layout = "2006-01-02T15:04:05.000000Z"
	parsed, err := time.Parse(layout, value)
	if err != nil || parsed.Format(layout) != value {
		return fmt.Errorf("%s must be UTC with exactly six fractional digits", field)
	}
	return nil
}

func validateCommandID(value string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return errors.New("command_id must be canonical unpadded base64url for 32 bytes")
	}
	return nil
}
