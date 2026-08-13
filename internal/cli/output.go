package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

type environment struct {
	defaultOutputRoot func() (string, error)
}

func productionEnvironment() environment {
	return environment{defaultOutputRoot: platform.DefaultOutputRoot}
}

func writeScanResult(stdout io.Writer, outcome agent.ScanOutcome, selected []string, outputRoot string, env environment) error {
	root := outputRoot
	if root == "" {
		var err error
		root, err = env.defaultOutputRoot()
		if err != nil {
			return err
		}
	}
	published, err := report.WriteBatch(root, outcome.Batch)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(published)
	if err != nil {
		return fmt.Errorf("解析批次路径: %w", err)
	}
	return writeScanSummary(stdout, outcome, selected, absolute)
}

func writeScanSummary(writer io.Writer, outcome agent.ScanOutcome, selected []string, published string) error {
	modules := strings.Join(selected, ", ")
	if outcome.Batch.Type == model.BatchTypeSnapshot {
		modules = "all"
	}
	if _, err := fmt.Fprintf(writer, "Asset Agent Scan\nModules: %s\nStatus: %s\n\n", modules, overallStatus(outcome.Batch.Results)); err != nil {
		return err
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "MODULE\tSTATUS\tRECORDS\tDURATION\tERROR"); err != nil {
		return err
	}
	for _, result := range outcome.Batch.Results {
		errorText := "-"
		if len(result.Errors) > 0 {
			errorText = compactError(result.Errors[0].Message)
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%d\t%dms\t%s\n", result.Module, result.Status,
			outcome.RecordCounts[result.Module], result.DurationMS, errorText); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "\nOutput: %s\n", published)
	return err
}

func overallStatus(results []model.ModuleResult) model.Status {
	rank := map[model.Status]int{model.StatusComplete: 0, model.StatusOK: 0, model.StatusDegraded: 1, model.StatusPartial: 2,
		model.StatusUnsupported: 3, model.StatusTimeout: 4, model.StatusFailed: 5}
	status := model.StatusComplete
	for _, result := range results {
		if rank[result.Status] > rank[status] {
			status = result.Status
		}
	}
	return status
}

func compactError(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if utf8.RuneCountInString(message) <= 120 {
		return message
	}
	runes := []rune(message)
	return string(runes[:117]) + "..."
}
