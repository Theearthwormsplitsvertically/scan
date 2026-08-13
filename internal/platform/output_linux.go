//go:build linux

package platform

// DefaultOutputRoot 返回 Linux 上供 CMDB 消费的固定输出根目录。
func DefaultOutputRoot() (string, error) {
	return "/var/lib/asset-agent/output", nil
}
