package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

type environment struct {
	executablePath func() (string, error)
	now            func() time.Time
}

func productionEnvironment() environment {
	return environment{executablePath: os.Executable, now: time.Now}
}

func writeScanResult(stdout io.Writer, batch model.Batch, target string, options scanOptions, env environment) error {
	if options.explicitFile {
		if target == "all" {
			return fmt.Errorf("all scan 不支持 -o/--output，请使用 --output-dir")
		}
		if options.output == "-" {
			return report.WriteJSON(stdout, batch)
		}
		if err := report.WriteJSONFile(options.output, batch); err != nil {
			return err
		}
		absolute, err := filepath.Abs(options.output)
		if err != nil {
			return fmt.Errorf("解析报告路径: %w", err)
		}
		_, err = fmt.Fprintln(stdout, absolute)
		return err
	}

	root := options.outputDir
	if !options.explicitDir {
		executable, err := env.executablePath()
		if err != nil {
			return fmt.Errorf("定位可执行文件: %w", err)
		}
		root, err = report.DefaultOutputRoot(executable)
		if err != nil {
			return err
		}
	}
	published, err := report.WriteBatch(root, batch)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, published)
	return err
}
