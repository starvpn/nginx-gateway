package geolite

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestEnsureBundledDBInstallsEmbeddedDatabase(t *testing.T) {
	originalConfPath := cSettings.ConfPath
	originalBundledDBXZ := bundledDBXZ
	t.Cleanup(func() {
		cSettings.ConfPath = originalConfPath
		bundledDBXZ = originalBundledDBXZ
	})

	var compressed bytes.Buffer
	writer, err := xz.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("failed to create xz writer: %v", err)
	}
	if _, err := writer.Write([]byte("test database")); err != nil {
		t.Fatalf("failed to write xz data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close xz writer: %v", err)
	}

	tempDir := t.TempDir()
	cSettings.ConfPath = filepath.Join(tempDir, "app.ini")
	bundledDBXZ = compressed.Bytes()

	if err := EnsureBundledDB(); err != nil {
		t.Fatalf("EnsureBundledDB() returned error: %v", err)
	}

	content, err := os.ReadFile(GetDBPath())
	if err != nil {
		t.Fatalf("failed to read installed database: %v", err)
	}
	if string(content) != "test database" {
		t.Fatalf("unexpected installed database content: %q", content)
	}
}

func TestEnsureBundledDBSkipsWhenUnavailable(t *testing.T) {
	originalConfPath := cSettings.ConfPath
	originalBundledDBXZ := bundledDBXZ
	t.Cleanup(func() {
		cSettings.ConfPath = originalConfPath
		bundledDBXZ = originalBundledDBXZ
	})

	tempDir := t.TempDir()
	cSettings.ConfPath = filepath.Join(tempDir, "app.ini")
	bundledDBXZ = nil

	if err := EnsureBundledDB(); err != nil {
		t.Fatalf("EnsureBundledDB() returned error: %v", err)
	}
	if DBExists() {
		t.Fatal("database should not be created when bundled data is unavailable")
	}
}
