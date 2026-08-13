package host

import "testing"

func TestParseMemoryBytesConvertsKiBAndRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	if got := ParseMemoryBytes([]byte("MemTotal:       2048 kB\nMemFree: 10 kB\n")); got != 2_097_152 {
		t.Fatalf("memory = %d, want 2097152", got)
	}
	if got := ParseMemoryBytes([]byte("MemTotal: broken kB\n")); got != 0 {
		t.Fatalf("malformed memory = %d, want 0", got)
	}
}
