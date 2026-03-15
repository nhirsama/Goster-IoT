package logger

import (
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const redactedValue = "***"

var sensitiveFieldKeywords = []string{
	"token",
	"secret",
	"password",
	"cookie",
	"authorization",
	"apikey",
	"api_key",
}

// RedactFields 对敏感字段进行脱敏，避免明文写入日志。
func RedactFields(fields ...inter.LogField) []inter.LogField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]inter.LogField, 0, len(fields))
	for _, f := range fields {
		if isSensitiveKey(f.Key) {
			out = append(out, inter.String(f.Key, redactedValue))
			continue
		}
		out = append(out, f)
	}
	return out
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, kw := range sensitiveFieldKeywords {
		if strings.Contains(k, kw) {
			return true
		}
	}
	return false
}
