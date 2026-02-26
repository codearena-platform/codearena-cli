package packager

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDir zips the contents of the given directory and returns it as a byte slice.
// Skips files common config/build files like .git or node_modules
func ZipDir(sourceDir string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip common unnecessary dirs
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "venv") {
			return filepath.SkipDir
		}

		// Relative path inside the zip
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Ensure it uses forward slashes in zip
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			if !strings.HasSuffix(relPath, "/") {
				relPath += "/"
			}
			_, err = zipWriter.Create(relPath)
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		w, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, f); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("falha ao zipar diretório: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("falha ao fechar o arquivo zip: %w", err)
	}

	return buf.Bytes(), nil
}
