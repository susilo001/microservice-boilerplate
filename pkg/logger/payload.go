package logger

import (
	"bytes"
	"encoding/json"
	"strings"

	"go.uber.org/zap"
)

func DebugPayload(log *zap.Logger, message, field string, value any) {
	if entry := log.Check(zap.DebugLevel, message); entry != nil {
		entry.Write(zap.Any(field, sanitizePayload(value)))
	}
}

func sanitizePayload(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{"omitted": true, "reason": "unserializable_payload"}
	}
	if len(data) > 16*1024 {
		return map[string]any{"omitted": true, "reason": "payload_too_large", "size_bytes": len(data)}
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return map[string]any{"omitted": true, "reason": "unserializable_payload"}
	}
	return redactPayload(decoded, 0)
}

func redactPayload(value any, depth int) any {
	if depth >= 8 {
		return "[MAX_DEPTH]"
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
			switch {
			case strings.Contains(normalized, "password"), strings.Contains(normalized, "secret"),
				strings.Contains(normalized, "token"), strings.Contains(normalized, "authorization"),
				strings.Contains(normalized, "cookie"), strings.Contains(normalized, "apikey"),
				strings.Contains(normalized, "privatekey"), strings.Contains(normalized, "accesskey"),
				normalized == "email", normalized == "phone", normalized == "ssn",
				normalized == "cardnumber", normalized == "cvv", normalized == "description",
				normalized == "credentials", normalized == "creditcard", normalized == "sessionid":
				typed[key] = "[REDACTED]"
			default:
				typed[key] = redactPayload(item, depth+1)
			}
		}
		return typed
	case []any:
		if len(typed) > 20 {
			typed = append(typed[:20], "[TRUNCATED]")
		}
		for index := range typed {
			typed[index] = redactPayload(typed[index], depth+1)
		}
		return typed
	default:
		return typed
	}
}
