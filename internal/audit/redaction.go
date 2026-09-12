package audit

const redactedAuditValue = "[REDACTED]"

var sensitiveAuditKeyFragments = [...]string{
	"authorization",
	"credential",
	"password",
	"private_key",
	"api_key",
	"apikey",
	"secret",
	"token",
	"proof",
	"cookie",
	"nonce",
}

var piiAuditKeyTokens = [...]string{
	"email",
	"phone",
	"address",
	"ip",
	"user_agent",
	"delegation_chain",
	"delegated_by",
	"creator_id",
}

func shouldRedactAuditContextKey(key string) bool {
	for _, fragment := range sensitiveAuditKeyFragments {
		if containsFoldASCII(key, fragment) {
			return true
		}
	}
	for _, token := range piiAuditKeyTokens {
		if hasDelimitedTokenFoldASCII(key, token) {
			return true
		}
	}
	return false
}

func redactedAuditContext(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		if shouldRedactAuditContextKey(key) {
			result[key] = redactedAuditValue
		} else {
			result[key] = value
		}
	}
	return result
}

func containsFoldASCII(value, fragment string) bool {
	if len(fragment) == 0 || len(fragment) > len(value) {
		return false
	}
	for start := 0; start <= len(value)-len(fragment); start++ {
		matched := true
		for i := 0; i < len(fragment); i++ {
			if lowerASCII(value[start+i]) != lowerASCII(fragment[i]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func hasDelimitedTokenFoldASCII(value, token string) bool {
	for start := 0; start <= len(value)-len(token); start++ {
		if start > 0 && !isAuditKeyDelimiter(value[start-1]) {
			continue
		}
		end := start + len(token)
		if end < len(value) && !isAuditKeyDelimiter(value[end]) {
			continue
		}
		matched := true
		for i := 0; i < len(token); i++ {
			if lowerASCII(value[start+i]) != lowerASCII(token[i]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func isAuditKeyDelimiter(value byte) bool {
	return value == '.' || value == '_' || value == '-'
}

func lowerASCII(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}
