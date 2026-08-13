package security

import (
	"strings"
	"testing"
)

func TestRedactArgsRemovesSecretsWithoutChangingArgumentCount(t *testing.T) {
	t.Parallel()

	input := []string{"server", "--password=hunter2", "--token", "abc123", "Authorization: Bearer jwt-value", "API_KEY=key-value", "https://user:pass@example.test/path", "--port=8080"}
	got := RedactArgs(input)
	joined := strings.Join(got, " ")

	if len(got) != len(input) {
		t.Fatalf("argument count = %d, want %d", len(got), len(input))
	}
	for _, secret := range []string{"hunter2", "abc123", "jwt-value", "key-value", "user:pass"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("redacted args contain %q: %q", secret, joined)
		}
	}
	if !strings.Contains(joined, "--port=8080") {
		t.Fatalf("ordinary option was changed: %q", joined)
	}
	if input[1] != "--password=hunter2" {
		t.Fatal("input mutated")
	}
}
