package radar

import (
	"os"
	"testing"
)

// TestMain keeps every desktop store of the tests out of the real ~/Documents: 工作目录 (and its 缓存/) default
// to a temp directory unless a test sets ARTIFACTS_DIR itself.
func TestMain(m *testing.M) {
	d, err := os.MkdirTemp("", "radar-artifacts-")
	if err != nil {
		panic(err)
	}
	if os.Getenv("ARTIFACTS_DIR") == "" {
		os.Setenv("ARTIFACTS_DIR", d)
	}
	privateOK = true // tests talk to httptest servers on 127.0.0.1; security_test.go turns the guard back on
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}
