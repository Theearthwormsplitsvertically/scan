package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

type batchWriteOptions struct {
	MaxRecords int
	MaxBytes   int64
	MaxLine    int
}

var defaultBatchWriteOptions = batchWriteOptions{
	MaxRecords: 100000,
	MaxBytes:   64 << 20,
	MaxLine:    1 << 20,
}

// WriteBatch 将批次以 JSONL 分片和最后写入的 manifest 原子发布到 inbox。
func WriteBatch(outputRoot string, batch model.Batch) (string, error) {
	return writeBatchWithOptions(outputRoot, batch, defaultBatchWriteOptions)
}

func writeBatchWithOptions(outputRoot string, batch model.Batch, options batchWriteOptions) (_ string, resultErr error) {
	if strings.TrimSpace(outputRoot) == "" {
		return "", fmt.Errorf("输出根目录不能为空")
	}
	if err := validateBatchWriteOptions(options); err != nil {
		return "", err
	}
	if err := validateFileComponent(batch.ID, "scan ID", true); err != nil {
		return "", err
	}
	formalName, err := batchDirectoryName(batch)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(outputRoot)
	if err != nil {
		return "", fmt.Errorf("解析输出根目录: %w", err)
	}
	if filepath.Dir(root) == root {
		return "", fmt.Errorf("输出根目录不能是文件系统卷根: %s", root)
	}
	if err := privateDirectory(root); err != nil {
		return "", fmt.Errorf("创建输出根目录: %w", err)
	}
	inbox := filepath.Join(root, "inbox")
	if err := privateDirectory(inbox); err != nil {
		return "", fmt.Errorf("创建 inbox: %w", err)
	}
	formalPath := filepath.Join(inbox, formalName)
	if _, err := os.Lstat(formalPath); err == nil {
		return "", fmt.Errorf("正式批次目录已存在: %s", formalPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("检查正式批次目录: %w", err)
	}
	partialPath := filepath.Join(inbox, ".partial-"+batch.ID)
	if err := os.Mkdir(partialPath, 0o700); err != nil {
		return "", fmt.Errorf("创建临时批次目录: %w", err)
	}
	partialCreated := true
	published := false
	defer func() {
		if partialCreated && !published {
			if cleanupErr := removeCurrentPartial(inbox, partialPath); resultErr == nil && cleanupErr != nil {
				resultErr = cleanupErr
			}
		}
	}()
	if err := os.Chmod(partialPath, 0o700); err != nil {
		return "", fmt.Errorf("设置临时批次目录权限: %w", err)
	}

	files := make([]model.BatchFile, 0)
	modules := make([]model.ModuleManifest, 0, len(batch.Results))
	for _, result := range batch.Results {
		modules = append(modules, moduleManifest(result))
		if !shouldPublishResult(batch, result) || len(result.Records) == 0 {
			continue
		}
		if err := validateFileComponent(result.Module, "module name", false); err != nil {
			return "", err
		}
		written, err := writeShardValues(partialPath, result.Module, result.Module, "asset_record", result.Records, options)
		if err != nil {
			return "", fmt.Errorf("写入模块 %s 分片: %w", result.Module, err)
		}
		files = append(files, written...)
	}

	relationshipWriter := newShardWriter(partialPath, "relationships", "", "relationship", options)
	defer relationshipWriter.abort()
	for _, result := range batch.Results {
		if !shouldPublishResult(batch, result) {
			continue
		}
		for _, relationship := range result.Relationships {
			if err := relationshipWriter.add(relationship); err != nil {
				return "", fmt.Errorf("写入关系分片: %w", err)
			}
		}
	}
	if err := relationshipWriter.closeCurrent(); err != nil {
		return "", fmt.Errorf("完成关系分片: %w", err)
	}
	files = append(files, relationshipWriter.files...)

	manifest := model.BatchManifest{
		SchemaName: model.BatchManifestSchemaName, SchemaVersion: model.BatchSchemaVersion,
		ScanID: batch.ID, BatchType: batch.Type, RequestedModule: batch.RequestedModule,
		Platform: batch.Platform, Agent: batch.Agent, StartedAt: batch.StartedAt, FinishedAt: batch.FinishedAt,
		Modules: modules, Files: files,
	}
	if err := writeSyncedJSON(filepath.Join(partialPath, "manifest.json"), manifest); err != nil {
		return "", fmt.Errorf("写入 manifest: %w", err)
	}
	if err := syncDirectory(partialPath); err != nil {
		return "", fmt.Errorf("同步临时批次目录: %w", err)
	}
	if err := os.Rename(partialPath, formalPath); err != nil {
		return "", fmt.Errorf("发布正式批次目录: %w", err)
	}
	published = true
	if err := syncDirectory(inbox); err != nil {
		return "", fmt.Errorf("同步 inbox: %w", err)
	}
	return formalPath, nil
}

func batchDirectoryName(batch model.Batch) (string, error) {
	started := batch.StartedAt.UTC()
	if started.IsZero() {
		started = time.Now().UTC()
	}
	timestamp := started.Format("20060102T150405Z")
	switch batch.Type {
	case model.BatchTypeSnapshot:
		return "snapshot-" + timestamp + "-" + batch.ID, nil
	case model.BatchTypeModule:
		if err := validateFileComponent(batch.RequestedModule, "requested module", false); err != nil {
			return "", err
		}
		return "module-" + batch.RequestedModule + "-" + timestamp + "-" + batch.ID, nil
	default:
		return "", fmt.Errorf("不支持的批次类型 %q", batch.Type)
	}
}

func validateBatchWriteOptions(options batchWriteOptions) error {
	if options.MaxRecords <= 0 || options.MaxBytes <= 0 || options.MaxLine <= 0 {
		return fmt.Errorf("分片限制必须大于零")
	}
	return nil
}

func validateFileComponent(value, label string, allowDot bool) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("%s 无效: %q", label, value)
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || (allowDot && character == '.') {
			continue
		}
		return fmt.Errorf("%s 包含不安全字符: %q", label, value)
	}
	return nil
}

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func shouldPublishResult(batch model.Batch, result model.ModuleResult) bool {
	return result.Published || batch.Type == model.BatchTypeSnapshot || result.Module == batch.RequestedModule
}

func moduleManifest(result model.ModuleResult) model.ModuleManifest {
	return model.ModuleManifest{
		Module: result.Module, SchemaVersion: result.SchemaVersion, Status: result.Status,
		Authoritative: result.Authoritative, StartedAt: result.StartedAt, FinishedAt: result.FinishedAt,
		DurationMS: result.DurationMS,
		Coverage: model.Coverage{
			ExpectedScopes:  append([]string{}, result.Coverage.ExpectedScopes...),
			CompletedScopes: append([]string{}, result.Coverage.CompletedScopes...),
			FailedScopes:    append([]string{}, result.Coverage.FailedScopes...),
		},
		Errors: append([]model.ErrorDetail{}, result.Errors...),
	}
}

func writeShardValues[T any](directory, prefix, moduleName, recordType string, values []T, options batchWriteOptions) ([]model.BatchFile, error) {
	writer := newShardWriter(directory, prefix, moduleName, recordType, options)
	defer writer.abort()
	for _, value := range values {
		if err := writer.add(value); err != nil {
			return nil, err
		}
	}
	if err := writer.closeCurrent(); err != nil {
		return nil, err
	}
	return writer.files, nil
}

type shardWriter struct {
	directory  string
	prefix     string
	module     string
	recordType string
	options    batchWriteOptions
	index      int
	file       *os.File
	digest     hash.Hash
	records    int
	bytes      int64
	name       string
	files      []model.BatchFile
}

func newShardWriter(directory, prefix, moduleName, recordType string, options batchWriteOptions) *shardWriter {
	return &shardWriter{
		directory: directory, prefix: prefix, module: moduleName, recordType: recordType,
		options: options, files: []model.BatchFile{},
	}
}

func (writer *shardWriter) add(value any) error {
	line, err := encodeJSONLine(value)
	if err != nil {
		return err
	}
	if len(line) > writer.options.MaxLine {
		return fmt.Errorf("单条 JSONL 记录为 %d 字节，超过上限 %d", len(line), writer.options.MaxLine)
	}
	if writer.file != nil && (writer.records >= writer.options.MaxRecords || writer.bytes+int64(len(line)) > writer.options.MaxBytes) {
		if err := writer.closeCurrent(); err != nil {
			return err
		}
	}
	if writer.file == nil {
		if err := writer.openNext(); err != nil {
			return err
		}
	}
	written, err := io.MultiWriter(writer.file, writer.digest).Write(line)
	writer.bytes += int64(written)
	if err != nil {
		return fmt.Errorf("写入 %s: %w", writer.name, err)
	}
	if written != len(line) {
		return io.ErrShortWrite
	}
	writer.records++
	return nil
}

func (writer *shardWriter) openNext() error {
	writer.index++
	writer.name = fmt.Sprintf("%s-%05d.jsonl", writer.prefix, writer.index)
	path := filepath.Join(writer.directory, writer.name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("创建分片 %s: %w", writer.name, err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("设置分片 %s 权限: %w", writer.name, err)
	}
	writer.file = file
	writer.digest = sha256.New()
	writer.records = 0
	writer.bytes = 0
	return nil
}

func (writer *shardWriter) closeCurrent() error {
	if writer.file == nil {
		return nil
	}
	file := writer.file
	writer.file = nil
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("同步分片 %s: %w", writer.name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭分片 %s: %w", writer.name, err)
	}
	writer.files = append(writer.files, model.BatchFile{
		Name: writer.name, Module: writer.module, RecordType: writer.recordType,
		Records: writer.records, Bytes: writer.bytes, SHA256: hex.EncodeToString(writer.digest.Sum(nil)),
	})
	writer.digest = nil
	return nil
}

func (writer *shardWriter) abort() {
	if writer.file != nil {
		_ = writer.file.Close()
		writer.file = nil
	}
}

func encodeJSONLine(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("编码 JSONL: %w", err)
	}
	return buffer.Bytes(), nil
}

func writeSyncedJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if err := WriteJSON(file, value); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func removeCurrentPartial(inbox, partial string) error {
	cleanInbox := filepath.Clean(inbox)
	cleanPartial := filepath.Clean(partial)
	if filepath.Dir(cleanPartial) != cleanInbox || !strings.HasPrefix(filepath.Base(cleanPartial), ".partial-") {
		return fmt.Errorf("拒绝清理非当前 inbox 临时目录: %s", partial)
	}
	if err := os.RemoveAll(cleanPartial); err != nil {
		return fmt.Errorf("清理临时批次目录: %w", err)
	}
	return nil
}
