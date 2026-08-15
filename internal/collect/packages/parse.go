package packages

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// descriptionLimit 限制单条包描述保留的字符数，避免异常字段撑大 JSONL 单行。
const descriptionLimit = 2048

// ParseDPKGStatus 解析 /var/lib/dpkg/status 的段落式包记录。
func ParseDPKGStatus(data []byte) ([]model.Package, error) {
	result := make([]model.Package, 0)
	current := map[string]string{}
	flush := func() {
		name := current["Package"]
		if name == "" {
			return
		}
		result = append(result, model.Package{
			Name:               name,
			Version:            current["Version"],
			Architecture:       current["Architecture"],
			Maintainer:         current["Maintainer"],
			Description:        firstDescriptionLine(current["Description"]),
			InstalledSizeBytes: installedSizeKB(current["Installed-Size"]),
		})
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			current = map[string]string{}
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			current["Description"] += "\n" + strings.TrimSpace(line)
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		current[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ParseAPKInstalled 解析 /lib/apk/db/installed 的字段式包记录。
func ParseAPKInstalled(data []byte) ([]model.Package, error) {
	result := make([]model.Package, 0)
	current := map[string]string{}
	flush := func() {
		name := current["P"]
		if name == "" {
			return
		}
		result = append(result, model.Package{
			Name:               name,
			Version:            current["V"],
			Architecture:       current["A"],
			Maintainer:         current["m"],
			Description:        firstDescriptionLine(current["T"]),
			InstalledSizeBytes: parseSizeBytes(current["S"]),
		})
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			current = map[string]string{}
			continue
		}
		if len(line) < 3 || line[1] != ':' {
			continue
		}
		current[line[:1]] = strings.TrimSpace(line[2:])
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// installedSizeKB 将 dpkg 的 Installed-Size（单位 kB）转换为字节数。
func installedSizeKB(value string) int64 {
	kb, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || kb <= 0 {
		return 0
	}
	return kb * 1024
}

// parseSizeBytes 解析 apk 的 S 字段（已是字节数）。
func parseSizeBytes(value string) int64 {
	bytesValue, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || bytesValue < 0 {
		return 0
	}
	return bytesValue
}

// firstDescriptionLine 只保留描述首行，并按上限截断。
func firstDescriptionLine(value string) string {
	line := value
	if index := strings.IndexAny(line, "\n"); index >= 0 {
		line = line[:index]
	}
	line = strings.TrimSpace(line)
	if len(line) > descriptionLimit {
		return line[:descriptionLimit]
	}
	return line
}
