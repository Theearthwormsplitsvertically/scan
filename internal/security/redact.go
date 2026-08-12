package security

import (
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveKeys = []string{"password", "passwd", "token", "secret", "authorization", "cookie", "api_key", "apikey", "private_key"}

func RedactArgs(args []string) []string {
	result := append([]string(nil), args...)
	redactNext := false
	for index, argument := range result {
		if redactNext {
			result[index] = redacted
			redactNext = false
			continue
		}
		lower := strings.ToLower(argument)
		if strings.HasPrefix(lower, "authorization:") {
			result[index] = "Authorization: " + redacted
			continue
		}
		if key, _, found := strings.Cut(argument, "="); found && isSensitiveKey(strings.TrimLeft(key, "-")) {
			result[index] = key + "=" + redacted
			continue
		}
		if strings.HasPrefix(argument, "-") && isSensitiveKey(strings.TrimLeft(argument, "-")) {
			redactNext = true
			continue
		}
		if parsed, err := url.Parse(argument); err == nil && parsed.User != nil {
			parsed.User = url.User(redacted)
			result[index] = parsed.String()
		}
	}
	return result
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, sensitive := range sensitiveKeys {
		if key == sensitive {
			return true
		}
	}
	return false
}
