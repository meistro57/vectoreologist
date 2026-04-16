package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeEnvFile creates a temp .env file with the given content and returns its path.
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatalf("could not write .env file: %v", err)
	}
	return f
}

// withCleanEnv removes the given keys from the environment before the test and
// restores them (or removes them) after.
func withCleanEnv(t *testing.T, keys ...string) {
	t.Helper()
	saved := make(map[string]string)
	wasSet := make(map[string]bool)
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		saved[k] = v
		wasSet[k] = ok
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if wasSet[k] {
				os.Setenv(k, saved[k])
			} else {
				os.Unsetenv(k)
			}
		}
	})
}

// ---- loadDotEnv tests -------------------------------------------------------

func TestLoadDotEnv_ParsesKeyValue(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_FOO")
	path := writeEnvFile(t, "TEST_DOTENV_FOO=bar\n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_FOO")
	if got != "bar" {
		t.Errorf("TEST_DOTENV_FOO: want %q, got %q", "bar", got)
	}
}

func TestLoadDotEnv_IgnoresCommentLines(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_COMMENT")
	path := writeEnvFile(t, "# this is a comment\nTEST_DOTENV_COMMENT=visible\n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_COMMENT")
	if got != "visible" {
		t.Errorf("TEST_DOTENV_COMMENT: want %q, got %q", "visible", got)
	}
}

func TestLoadDotEnv_IgnoresEmptyLines(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_EMPTY")
	path := writeEnvFile(t, "\n\nTEST_DOTENV_EMPTY=present\n\n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_EMPTY")
	if got != "present" {
		t.Errorf("TEST_DOTENV_EMPTY: want %q, got %q", "present", got)
	}
}

func TestLoadDotEnv_StripsDoubleQuotes(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_DQUOTE")
	path := writeEnvFile(t, `TEST_DOTENV_DQUOTE="hello world"`)

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_DQUOTE")
	if got != "hello world" {
		t.Errorf("TEST_DOTENV_DQUOTE: want %q, got %q", "hello world", got)
	}
}

func TestLoadDotEnv_StripsSingleQuotes(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_SQUOTE")
	path := writeEnvFile(t, "TEST_DOTENV_SQUOTE='hello world'")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_SQUOTE")
	if got != "hello world" {
		t.Errorf("TEST_DOTENV_SQUOTE: want %q, got %q", "hello world", got)
	}
}

func TestLoadDotEnv_DoesNotOverrideExistingVar(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_NOOVER")
	os.Setenv("TEST_DOTENV_NOOVER", "original")
	path := writeEnvFile(t, "TEST_DOTENV_NOOVER=overridden\n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_NOOVER")
	if got != "original" {
		t.Errorf("existing env var must not be overridden: want %q, got %q", "original", got)
	}
}

func TestLoadDotEnv_MissingFileIsNotAnError(t *testing.T) {
	// A missing .env file should be silently ignored.
	// If this panics or returns an error we would see a test failure.
	loadDotEnv("/tmp/this-file-does-not-exist-vectoreologist-test.env")
}

func TestLoadDotEnv_TrimsWhitespaceAroundKey(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_TRIM")
	path := writeEnvFile(t, "  TEST_DOTENV_TRIM  =  trimmed  \n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_TRIM")
	if got != "trimmed" {
		t.Errorf("TEST_DOTENV_TRIM: want %q, got %q", "trimmed", got)
	}
}

func TestLoadDotEnv_SkipsLinesWithoutEquals(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_NOEQUALS", "NODEQUALS")
	path := writeEnvFile(t, "NODEQUALS\nTEST_DOTENV_NOEQUALS=ok\n")

	loadDotEnv(path)

	// NODEQUALS has no '=' → must NOT be set.
	if _, ok := os.LookupEnv("NODEQUALS"); ok {
		t.Error("line without '=' should not be set as env var")
	}
	// TEST_DOTENV_NOEQUALS should be set normally.
	if got := os.Getenv("TEST_DOTENV_NOEQUALS"); got != "ok" {
		t.Errorf("TEST_DOTENV_NOEQUALS: want %q, got %q", "ok", got)
	}
}

func TestLoadDotEnv_MultipleVarsInFile(t *testing.T) {
	withCleanEnv(t, "TEST_DOTENV_A", "TEST_DOTENV_B", "TEST_DOTENV_C")
	content := "TEST_DOTENV_A=one\nTEST_DOTENV_B=two\nTEST_DOTENV_C=three\n"
	path := writeEnvFile(t, content)

	loadDotEnv(path)

	want := map[string]string{
		"TEST_DOTENV_A": "one",
		"TEST_DOTENV_B": "two",
		"TEST_DOTENV_C": "three",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s: want %q, got %q", k, v, got)
		}
	}
}

func TestLoadDotEnv_ValueContainsEquals(t *testing.T) {
	// Key=val=extra → key="Key", val="val=extra" (strings.Cut splits on first =).
	withCleanEnv(t, "TEST_DOTENV_EQ")
	path := writeEnvFile(t, "TEST_DOTENV_EQ=a=b=c\n")

	loadDotEnv(path)

	got := os.Getenv("TEST_DOTENV_EQ")
	if got != "a=b=c" {
		t.Errorf("value with embedded '=': want %q, got %q", "a=b=c", got)
	}
}

func TestLoadDotEnv_InlineCommentNotStripped(t *testing.T) {
	// The implementation does NOT strip inline comments (no special handling).
	// The raw value "value # comment" (minus surrounding whitespace) should be stored.
	withCleanEnv(t, "TEST_DOTENV_INLINE")
	path := writeEnvFile(t, "TEST_DOTENV_INLINE=value # comment\n")

	loadDotEnv(path)

	// After TrimSpace the value will be "value # comment" (quotes are stripped,
	// but inline comments are not specially handled).
	got := os.Getenv("TEST_DOTENV_INLINE")
	if got != "value # comment" {
		t.Errorf("inline comment: want %q, got %q", "value # comment", got)
	}
}

// ---- watch mode flag tests --------------------------------------------------

func TestWatchDurationValid(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"30s", 30 * time.Second},
		{"1h30m", 90 * time.Minute},
		{"100ms", 100 * time.Millisecond},
	}
	for _, tc := range cases {
		d, err := time.ParseDuration(tc.input)
		if err != nil {
			t.Errorf("ParseDuration(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if d != tc.want {
			t.Errorf("ParseDuration(%q) = %v; want %v", tc.input, d, tc.want)
		}
		if d <= 0 {
			t.Errorf("ParseDuration(%q) = %v; want positive", tc.input, d)
		}
	}
}

func TestWatchDurationInvalid(t *testing.T) {
	cases := []string{
		"5 minutes",
		"five",
		"1d", // Go does not support day units
		"1w",
	}
	for _, s := range cases {
		_, err := time.ParseDuration(s)
		if err == nil {
			t.Errorf("ParseDuration(%q) expected error, got nil", s)
		}
	}
}

func TestWatchDurationNonPositive(t *testing.T) {
	cases := []struct {
		input   string
		wantPos bool
	}{
		{"-1m", false},
		{"-30s", false},
		{"0s", false},
		{"1s", true},
	}
	for _, tc := range cases {
		d, err := time.ParseDuration(tc.input)
		if err != nil {
			continue
		}
		isPos := d > 0
		if isPos != tc.wantPos {
			t.Errorf("ParseDuration(%q) = %v; positive=%v, want positive=%v", tc.input, d, isPos, tc.wantPos)
		}
	}
}

// ---- batch size clamping tests ----------------------------------------------

func TestBatchSizeClamping(t *testing.T) {
	cases := []struct {
		sample    int
		batch     int
		wantBatch int
	}{
		{5000, 5000, 5000}, // equal — no clamp
		{100, 5000, 100},   // batch > sample — clamp to sample
		{1000, 500, 500},   // batch < sample — no change
		{1, 1, 1},          // minimum
	}
	for _, tc := range cases {
		b := tc.batch
		if b > tc.sample {
			b = tc.sample
		}
		if b != tc.wantBatch {
			t.Errorf("clamp(sample=%d, batch=%d) = %d; want %d", tc.sample, tc.batch, b, tc.wantBatch)
		}
	}
}
