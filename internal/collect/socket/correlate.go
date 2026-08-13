package socket

import (
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// FDSource 是 socket 归属关联所需的最小 procfs 文件描述符视图。
type FDSource interface {
	ReadDir(string) ([]fs.DirEntry, error)
	Readlink(string) (string, error)
}

// Correlate 使用 /proc/<pid>/fd 符号链接将 socket inode 关联到进程身份。
// 只有严格的 socket:[inode] 证据才会生成 socket_process 关系。
func Correlate(source FDSource, sockets []model.Socket, processes []model.Process) ([]model.Socket, []model.Relationship) {
	byInode := make(map[uint64]int, len(sockets))
	for index := range sockets {
		byInode[sockets[index].Inode] = index
	}
	relationships := make([]model.Relationship, 0)
	observedAt := time.Now().UTC()
	seen := make(map[string]struct{})
	for _, process := range processes {
		directory := fmt.Sprintf("/proc/%d/fd", process.PID)
		entries, err := source.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := directory + "/" + entry.Name()
			target, err := source.Readlink(path)
			if err != nil {
				continue
			}
			inode, ok := parseSocketLink(target)
			if !ok {
				continue
			}
			index, exists := byInode[inode]
			if !exists {
				continue
			}
			key := fmt.Sprintf("%d:%s", inode, process.ID)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			sockets[index].PIDs = append(sockets[index].PIDs, process.PID)
			sockets[index].ProcessIDs = append(sockets[index].ProcessIDs, process.ID)
			relationships = append(relationships, model.Relationship{ID: "socket_process:" + key, Type: "socket_process", FromID: sockets[index].ID, ToID: process.ID, Source: path, Collector: "socket", Confidence: "exact", ObservedAt: observedAt})
		}
	}
	for index := range sockets {
		sort.Ints(sockets[index].PIDs)
		sort.Strings(sockets[index].ProcessIDs)
	}
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].ID < relationships[j].ID })
	return sockets, relationships
}

// parseSocketLink 只接受精确的 Linux socket 文件描述符链接格式。
func parseSocketLink(value string) (uint64, bool) {
	if !strings.HasPrefix(value, "socket:[") || !strings.HasSuffix(value, "]") {
		return 0, false
	}
	inode, err := strconv.ParseUint(value[len("socket:["):len(value)-1], 10, 64)
	return inode, err == nil
}
