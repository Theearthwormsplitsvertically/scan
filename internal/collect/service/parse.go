package service

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// ParseUnit 解析一个 systemd Unit 文件的静态事实。
// 只提取 [Unit] Description、[Service] ExecStart/User/Group、[Install] WantedBy。
func ParseUnit(data []byte, unitName string) (model.Service, error) {
	result := model.Service{UnitName: unitName}
	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch section {
		case "unit":
			if key == "Description" {
				result.Description = value
			}
		case "service":
			switch key {
			case "ExecStart":
				if result.ExecStart == "" {
					result.ExecStart = value
				}
			case "User":
				result.User = value
			case "Group":
				result.Group = value
			}
		case "install":
			if key == "WantedBy" {
				result.WantedBy = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return model.Service{}, err
	}
	return result, nil
}
