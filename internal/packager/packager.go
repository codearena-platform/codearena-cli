// Package packager builds the zip payload uploaded by the CLI.
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

const (
	// MaxPayloadBytes caps the total uncompressed size of a bot upload.
	MaxPayloadBytes = 10 << 20
	// MaxFileBytes caps the size of a single source file.
	MaxFileBytes = 2 << 20
	// MaxFiles caps the number of files in a payload.
	MaxFiles = 500
)

var allowedExtensions = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".java": true,
	".json": true, ".yaml": true, ".yml": true,
	".md": true, ".txt": true,
}

var deniedFiles = map[string]bool{
	".env": true, ".npmrc": true, ".pypirc": true, ".netrc": true,
	".htpasswd": true, ".git-credentials": true, "credentials.json": true,
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
}

var deniedDirs = map[string]bool{
	".git": true, "node_modules": true, "venv": true, ".venv": true,
	"dist": true, "build": true, "out": true, "bin": true,
	".idea": true, ".vscode": true, ".codearena": true, "__pycache__": true,
}

// ZipDir zips the allowed source files of sourceDir. Symlinks, secret files and
// build/dependency directories are skipped, payload size is capped, and every
// file handle is closed as soon as its contents have been copied.
func ZipDir(sourceDir string) ([]byte, error) {
	info, err := os.Lstat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("acessar diretório: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("diretório de origem não pode ser um symlink: %s", sourceDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("origem não é um diretório: %s", sourceDir)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	var totalBytes int64
	var fileCount int
	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// filepath.Walk uses Lstat: symlinked files and directories are never
		// followed. Skip them explicitly so they cannot leak outside the tree.
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if path != sourceDir && skipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || !allowedSourceFile(info.Name()) {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("calcular caminho relativo: %w", err)
		}
		if relPath == "." || filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("caminho de arquivo inválido no pacote: %s", relPath)
		}
		if fileCount >= MaxFiles {
			return fmt.Errorf("pacote excede o limite de %d arquivos", MaxFiles)
		}

		w, err := zipWriter.Create(filepath.ToSlash(relPath))
		if err != nil {
			return fmt.Errorf("adicionar %s ao zip: %w", relPath, err)
		}
		written, err := copyFile(w, path, MaxFileBytes)
		if err != nil {
			return err
		}
		totalBytes += written
		if totalBytes > MaxPayloadBytes {
			return fmt.Errorf("pacote excede o limite de %d bytes", MaxPayloadBytes)
		}
		fileCount++
		return nil
	})

	closeErr := zipWriter.Close()
	if walkErr != nil {
		return nil, fmt.Errorf("falha ao zipar diretório: %w", walkErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("falha ao fechar o arquivo zip: %w", closeErr)
	}
	return buf.Bytes(), nil
}

// copyFile copies at most limit+1 bytes, so oversized files are detected
// without buffering unbounded data. The file is closed on every path.
func copyFile(dst io.Writer, path string, limit int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("abrir %s: %w", path, err)
	}
	defer f.Close()

	written, err := io.Copy(dst, io.LimitReader(f, limit+1))
	if err != nil {
		return written, fmt.Errorf("copiar %s: %w", path, err)
	}
	if written > limit {
		return written, fmt.Errorf("arquivo %s excede o limite de %d bytes", path, limit)
	}
	return written, nil
}

func skipDir(name string) bool {
	if deniedDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
		return true
	}
	return false
}

func allowedSourceFile(name string) bool {
	lower := strings.ToLower(name)
	if deniedFiles[lower] || strings.HasPrefix(lower, ".env") {
		return false
	}
	for _, fragment := range []string{"secret", "credential", "password", "token"} {
		if strings.Contains(lower, fragment) {
			return false
		}
	}
	switch filepath.Ext(lower) {
	case ".pem", ".key", ".p12", ".pfx", ".crt", ".cer", ".jks", ".keystore", ".log":
		return false
	}
	return allowedExtensions[filepath.Ext(lower)]
}
