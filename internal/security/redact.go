// security 包提供采集结果输出前的安全处理辅助函数。
package security

import (
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveKeys = []string{"password", "passwd", "token", "secret", "authorization", "cookie", "api_key", "apikey", "private_key"}

// RedactArgs 返回已移除凭据类值的 args 副本。
// 它保留参数数量，使消费者仍能理解原始命令行结构。
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

// isSensitiveKey 在去掉选项前缀后识别支持的敏感凭据键名。
func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, sensitive := range sensitiveKeys {
		if key == sensitive {
			return true
		}
	}
	return false
}
