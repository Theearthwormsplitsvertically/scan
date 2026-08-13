package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

func TestWriteBatchPublishesAtomicVerifiableDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "output")
	batch := testBatch("scan-1", []model.AssetRecord{{
		RecordID: "host:1", RecordType: "host", HostID: "host:1",
		ScopeID: "host:1", ScopeType: "host", Name: "server-1", Platform: "linux",
		Evidence: []model.Evidence{},
	}})
	batch.Results[0].Relationships = []model.RelationshipRecord{{
		RecordID: "relationship:1", RelationshipType: "runs", FromID: "host:1", ToID: "process:1",
		ObservedAt: batch.StartedAt, Confidence: "exact", Evidence: []model.Evidence{},
	}}

	published, err := WriteBatch(root, batch)
	if err != nil {
		t.Fatal(err)
	}
	wantDirectory := filepath.Join(root, "inbox", "module-host-20260813T020000Z-scan-1")
	if published != wantDirectory {
		t.Fatalf("published = %q, want %q", published, wantDirectory)
	}
	assertMode(t, root, 0o700)
	assertMode(t, filepath.Join(root, "inbox"), 0o700)
	assertMode(t, filepath.Join(published, "manifest.json"), 0o600)
	assertMode(t, filepath.Join(published, "host-00001.jsonl"), 0o600)
	assertMode(t, filepath.Join(published, "relationships-00001.jsonl"), 0o600)

	manifestData, err := os.ReadFile(filepath.Join(published, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest model.BatchManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaName != model.BatchManifestSchemaName || manifest.SchemaVersion != model.BatchSchemaVersion {
		t.Fatalf("manifest = %+v", manifest)
	}
	if len(manifest.Modules) != 1 || len(manifest.Files) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, file := range manifest.Files {
		data, err := os.ReadFile(filepath.Join(published, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if file.Bytes != int64(len(data)) || file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("file metadata = %+v, bytes = %d", file, len(data))
		}
		if file.Records != strings.Count(string(data), "\n") {
			t.Fatalf("file record count = %+v", file)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".partial-") {
			t.Fatalf("partial directory remains: %s", entry.Name())
		}
	}
}

func TestWriteBatchSplitsByRecordLimit(t *testing.T) {
	t.Parallel()

	records := make([]model.AssetRecord, 0, 5)
	for index := 0; index < 5; index++ {
		records = append(records, model.AssetRecord{
			RecordID: "host:" + string(rune('a'+index)), RecordType: "host", HostID: "host:1",
			ScopeID: "host:1", ScopeType: "host", Name: "server", Platform: "linux", Evidence: []model.Evidence{},
		})
	}
	published, err := writeBatchWithOptions(filepath.Join(t.TempDir(), "output"), testBatch("scan-shards", records), batchWriteOptions{
		MaxRecords: 2, MaxBytes: 64 << 20, MaxLine: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, published)
	if len(manifest.Files) != 3 {
		t.Fatalf("files = %+v", manifest.Files)
	}
	wantNames := []string{"host-00001.jsonl", "host-00002.jsonl", "host-00003.jsonl"}
	wantRecords := []int{2, 2, 1}
	for index, file := range manifest.Files {
		if file.Name != wantNames[index] || file.Records != wantRecords[index] {
			t.Fatalf("file %d = %+v", index, file)
		}
	}
}

func TestWriteBatchEncodingFailureCleansOnlyCurrentPartialDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "output")
	existing := filepath.Join(root, "inbox", "snapshot-existing")
	if err := os.MkdirAll(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existing, "manifest.json")
	if err := os.WriteFile(marker, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	batch := testBatch("scan-bad", []model.AssetRecord{{
		RecordID: "host:bad", RecordType: "host", HostID: "host:bad",
		ScopeID: "host:bad", ScopeType: "host", Name: "bad", Platform: "linux",
		Attributes: map[string]any{"invalid": make(chan int)}, Evidence: []model.Evidence{},
	}})

	if _, err := WriteBatch(root, batch); err == nil {
		t.Fatal("WriteBatch accepted a non-JSON value")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "old" {
		t.Fatalf("pre-existing batch changed: data=%q err=%v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "snapshot-existing" {
		t.Fatalf("inbox entries = %+v", entries)
	}
}

func TestWriteBatchDoesNotOverwriteFormalBatch(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "output")
	batch := testBatch("scan-conflict", []model.AssetRecord{{
		RecordID: "host:1", RecordType: "host", HostID: "host:1",
		ScopeID: "host:1", ScopeType: "host", Name: "server", Platform: "linux", Evidence: []model.Evidence{},
	}})
	first, err := WriteBatch(root, batch)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(first, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteBatch(root, batch); err == nil {
		t.Fatal("existing formal batch was overwritten")
	}
	after, err := os.ReadFile(filepath.Join(first, "manifest.json"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("formal batch changed: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "inbox", ".partial-scan-conflict")); !os.IsNotExist(err) {
		t.Fatalf("partial directory remains: %v", err)
	}
}

func TestWriteBatchRejectsUnsafeScanID(t *testing.T) {
	t.Parallel()

	batch := testBatch("../outside", []model.AssetRecord{})
	if _, err := WriteBatch(filepath.Join(t.TempDir(), "output"), batch); err == nil {
		t.Fatal("unsafe scan ID accepted")
	}
}

func TestWriteBatchRejectsEmptyOutputRoot(t *testing.T) {
	t.Parallel()

	if _, err := WriteBatch("", testBatch("scan-empty-root", []model.AssetRecord{})); err == nil {
		t.Fatal("empty output root accepted")
	}
}

func testBatch(scanID string, records []model.AssetRecord) model.Batch {
	started := time.Date(2026, time.August, 13, 2, 0, 0, 0, time.UTC)
	return model.Batch{
		SchemaName: model.BatchSchemaName, SchemaVersion: model.BatchSchemaVersion,
		ID: scanID, Type: model.BatchTypeModule, RequestedModule: "host", Platform: "linux",
		Agent: model.AgentInfo{Name: "asset-agent", Version: "test"}, StartedAt: started, FinishedAt: started.Add(time.Second),
		Results: []model.ModuleResult{{
			Module: "host", SchemaVersion: model.BatchSchemaVersion, Status: model.StatusComplete,
			Authoritative: true, StartedAt: started, FinishedAt: started.Add(time.Second), DurationMS: 1000,
			Coverage: model.Coverage{ExpectedScopes: []string{"host:1"}, CompletedScopes: []string{"host:1"}, FailedScopes: []string{}},
			Errors:   []model.ErrorDetail{}, Records: records, Relationships: []model.RelationshipRecord{}, Published: true,
		}},
	}
}

func readManifest(t *testing.T, directory string) model.BatchManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest model.BatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %s = %o, want %o", path, got, want)
	}
}
