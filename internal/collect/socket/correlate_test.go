package socket

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

type fakeFDSource struct {
	entries map[string][]fs.DirEntry
	links   map[string]string
}

func (source fakeFDSource) ReadDir(path string) ([]fs.DirEntry, error) {
	entries, exists := source.entries[path]
	if !exists {
		return nil, errors.New("missing")
	}
	return entries, nil
}

func (source fakeFDSource) Readlink(path string) (string, error) {
	link, exists := source.links[path]
	if !exists {
		return "", errors.New("vanished")
	}
	return link, nil
}

type fakeDirEntry string

func (entry fakeDirEntry) Name() string         { return string(entry) }
func (fakeDirEntry) IsDir() bool                { return false }
func (fakeDirEntry) Type() fs.FileMode          { return 0 }
func (fakeDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("unused") }

func TestCorrelateMapsSharedSocketToBothProcesses(t *testing.T) {
	t.Parallel()

	processes := []model.Process{{ID: "boot:10:1", PID: 10}, {ID: "boot:20:2", PID: 20}}
	sockets := []model.Socket{{ID: "net:[1]:12345:tcp", Inode: 12345, Protocol: "tcp", PIDs: []int{}, ProcessIDs: []string{}}}
	source := fakeFDSource{
		entries: map[string][]fs.DirEntry{"/proc/10/fd": {fakeDirEntry("3"), fakeDirEntry("4")}, "/proc/20/fd": {fakeDirEntry("8")}},
		links:   map[string]string{"/proc/10/fd/3": "socket:[12345]", "/proc/10/fd/4": "/tmp/file", "/proc/20/fd/8": "socket:[12345]"},
	}

	gotSockets, relationships := Correlate(source, sockets, processes)

	if len(gotSockets[0].PIDs) != 2 || gotSockets[0].PIDs[0] != 10 || gotSockets[0].PIDs[1] != 20 {
		t.Fatalf("PIDs = %#v", gotSockets[0].PIDs)
	}
	if len(relationships) != 2 || relationships[0].Type != "socket_process" || relationships[0].Confidence != "exact" {
		t.Fatalf("relationships = %+v", relationships)
	}
}
