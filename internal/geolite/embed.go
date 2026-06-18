package geolite

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ulikunitz/xz"
)

const bundledDBXZPath = "assets/GeoLite2-City.mmdb.xz"

//go:embed assets/*
var bundledFS embed.FS

var bundledDBXZ []byte

var bundledInstallMu sync.Mutex

// BundledDBAvailable reports whether the build contains the GeoLite2 database.
func BundledDBAvailable() bool {
	data, err := readBundledDBXZ()
	return err == nil && len(data) > 0
}

// EnsureBundledDB installs the embedded GeoLite2 database when the runtime copy is missing.
func EnsureBundledDB() error {
	bundledInstallMu.Lock()
	defer bundledInstallMu.Unlock()

	if DBExists() {
		return nil
	}

	data, err := readBundledDBXZ()
	if err != nil || len(data) == 0 {
		return nil
	}

	reader, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to read bundled GeoLite2 database: %w", err)
	}

	dbPath := GetDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create GeoLite2 database directory: %w", err)
	}

	file, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create GeoLite2 database: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(dbPath)
		return fmt.Errorf("failed to install bundled GeoLite2 database: %w", err)
	}

	return nil
}

func readBundledDBXZ() ([]byte, error) {
	if bundledDBXZ != nil {
		return bundledDBXZ, nil
	}
	return bundledFS.ReadFile(bundledDBXZPath)
}
