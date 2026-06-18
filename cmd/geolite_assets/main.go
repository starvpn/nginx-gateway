//go:generate go run .

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	downloadURL = "http://cloud.nginxui.com/geolite/GeoLite2-City.mmdb.xz"
	outputPath  = "../../internal/geolite/assets/GeoLite2-City.mmdb.xz"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare GeoLite2 database: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %s", resp.Status)
	}

	tempPath := outputPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(tempPath)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tempPath)
		return closeErr
	}

	return os.Rename(tempPath, outputPath)
}
