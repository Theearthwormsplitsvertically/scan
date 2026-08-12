package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

type environment struct {
	executablePath func() (string, error)
	now            func() time.Time
}

func productionEnvironment() environment {
	return environment{executablePath: os.Executable, now: time.Now}
}

func writeScanResult(stdout io.Writer, value any, module, requestedPath string, explicit bool, env environment) error {
	if explicit && requestedPath == "-" {
		return report.WriteJSON(stdout, value)
	}
	path := requestedPath
	if !explicit {
		executable, err := env.executablePath()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		path, err = report.DefaultOutputPath(executable, module, env.now())
		if err != nil {
			return err
		}
	}
	if err := report.WriteJSONFile(path, value); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve report path: %w", err)
	}
	_, err = fmt.Fprintln(stdout, absolute)
	return err
}
