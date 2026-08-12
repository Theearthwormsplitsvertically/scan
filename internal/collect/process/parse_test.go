package process

import "testing"

func TestParseStatHandlesCommandWithSpacesAndParentheses(t *testing.T) {
	t.Parallel()

	line := []byte("321 (worker (blue) pool) S 7 1 1 0 0 0 0 0 0 0 0 0 0 0 20 0 1 0 98765 0 0")
	got, err := ParseStat(line)
	if err != nil {
		t.Fatalf("ParseStat() error = %v", err)
	}
	if got.PID != 321 || got.Command != "worker (blue) pool" || got.State != "S" || got.PPID != 7 || got.StartTime != 98765 {
		t.Fatalf("stat = %+v", got)
	}
}

func TestParseStatRejectsTruncatedInput(t *testing.T) {
	t.Parallel()

	if _, err := ParseStat([]byte("1 (init) S 0")); err == nil {
		t.Fatal("ParseStat() error = nil, want truncation error")
	}
}

func TestParseCmdlinePreservesArguments(t *testing.T) {
	t.Parallel()

	got := ParseCmdline([]byte("nginx\x00-g\x00daemon off;\x00"))
	if len(got) != 3 || got[1] != "-g" || got[2] != "daemon off;" {
		t.Fatalf("cmdline = %#v", got)
	}
}
