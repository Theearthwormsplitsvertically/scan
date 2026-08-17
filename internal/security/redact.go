// security 包提供采集结果输出前的安全处理辅助函数。
package security

import (
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveKeyTokens = map[string]bool{
	"password": true, "passwd": true, "token": true, "secret": true,
	"authorization": true, "cookie": true, "credential": true, "credentials": true,
	"apikey": true, "privatekey": true,
}

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
		if value, changed := redactURL(argument); changed {
			result[index] = value
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
	}
	return result
}

// RedactLabels 返回已脱敏的标签副本，敏感键对应的值替换为 [REDACTED]。
// 容器/镜像标签常被塞入凭据，须在进入统一模型前完成脱敏。
func RedactLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		if isSensitiveKey(key) {
			result[key] = redacted
		} else {
			result[key] = value
		}
	}
	return result
}

// isSensitiveKey 识别精确键和由 -、_、. 组合的凭据键，同时保留 -p 等含义不明确的短参数。
func isSensitiveKey(key string) bool {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "-D") || strings.HasPrefix(key, "-d") {
		key = key[2:]
	} else {
		key = strings.TrimLeft(key, "-")
	}
	tokens := strings.FieldsFunc(strings.ToLower(key), func(character rune) bool {
		return character == '-' || character == '_' || character == '.'
	})
	for index, token := range tokens {
		if sensitiveKeyTokens[token] {
			return true
		}
		if index+1 < len(tokens) && ((token == "api" && tokens[index+1] == "key") ||
			(token == "private" && tokens[index+1] == "key")) {
			return true
		}
	}
	return false
}

func redactURL(argument string) (string, bool) {
	parsed, err := url.Parse(argument)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return argument, false
	}
	changed := false
	if parsed.User != nil {
		parsed.User = url.User(redacted)
		changed = true
	}
	query := parsed.Query()
	for key := range query {
		if isSensitiveKey(key) {
			query.Set(key, redacted)
			changed = true
		}
	}
	if changed {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), changed
}
