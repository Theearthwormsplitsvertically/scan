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

func TestRedactArgsCompositeCredentialKeysAndPreservesAmbiguousShortOption(t *testing.T) {
	t.Parallel()

	input := []string{
		"--db-password=db-secret",
		"--client_secret", "client-secret",
		"-Dfoo.password=java-secret",
		"--password-file=/run/secrets/db",
		"DB_ACCESS_TOKEN=env-secret",
		"https://example.test/api?access_token=url-secret&port=443",
		"-p", "5432",
	}
	got := RedactArgs(input)
	joined := strings.Join(got, " ")

	if len(got) != len(input) {
		t.Fatalf("argument count = %d, want %d", len(got), len(input))
	}
	for _, secret := range []string{"db-secret", "client-secret", "java-secret", "/run/secrets/db", "env-secret", "url-secret"} {
		if strings.Contains(joined, secret) {
			t.Errorf("redacted args contain %q: %q", secret, joined)
		}
	}
	if !strings.Contains(joined, "-p 5432") {
		t.Fatalf("ambiguous short option was redacted: %q", joined)
	}
	if !strings.Contains(joined, "port=443") {
		t.Fatalf("ordinary URL query parameter was changed: %q", joined)
	}
}

func TestRedactLabelsRemovesSensitiveValuesOnly(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"com.example.app":   "nginx",
		"com.example.token": "secret-token",
		"DB_PASSWORD":       "hunter2",
		"maintainer":        "ops@example.com",
	}
	got := RedactLabels(labels)
	if got["com.example.app"] != "nginx" {
		t.Fatalf("ordinary label changed: %q", got["com.example.app"])
	}
	if got["com.example.token"] != redacted || got["DB_PASSWORD"] != redacted {
		t.Fatalf("sensitive label not redacted: %+v", got)
	}
	if got["maintainer"] != "ops@example.com" {
		t.Fatalf("ordinary label changed: %q", got["maintainer"])
	}
	if RedactLabels(nil) != nil {
		t.Fatal("nil labels should return nil")
	}
}
