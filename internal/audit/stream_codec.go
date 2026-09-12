package audit

import (
	"encoding/json"
	"errors"
	"time"
)

// escapeJSON encodes the JSON string characters used by the allocation-free stream writer.
func escapeJSON(dst []byte, value string) []byte {
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, value[i])
		}
	}
	return dst
}

// DecodeNDJSONLogEntry decodes one audit record for tests and diagnostics.
func DecodeNDJSONLogEntry(data []byte) (*LogEntry, error) {
	if len(data) == 0 {
		return nil, errors.New("du lieu ndjson rong")
	}

	var entry LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.Timestamp > 0 {
		entry.EvaluatedAt = time.Unix(0, entry.Timestamp).UTC()
	}
	return &entry, nil
}

// DecodeBinaryLogEntry is retained for backward compatibility.
func DecodeBinaryLogEntry(data []byte) (*LogEntry, error) {
	return DecodeNDJSONLogEntry(data)
}
