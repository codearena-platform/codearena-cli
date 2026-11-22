package packager

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func zipEntries(t *testing.T, payload []byte) map[string]bool {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	entries := make(map[string]bool)
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	return entries
}

func TestZipDirAllowListAndSymlinks(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "bot")
	outside := filepath.Join(root, "outside")
	writeFile(t, filepath.Join(src, "bot.ts"), "export const bot = 1;")
	writeFile(t, filepath.Join(src, "codearena.yaml"), "name: bot")
	writeFile(t, filepath.Join(src, "package.json"), `{"name":"bot"}`)
	writeFile(t, filepath.Join(src, "README.md"), "docs")
	writeFile(t, filepath.Join(src, "src", "helper.ts"), "export const helper = 2;")
	writeFile(t, filepath.Join(src, ".env"), "SECRET=do-not-ship")
	writeFile(t, filepath.Join(src, "credentials.json"), `{"token":"do-not-ship"}`)
	writeFile(t, filepath.Join(src, "server.key"), "private key")
	writeFile(t, filepath.Join(src, ".codearena", "config.json"), `{"token":"do-not-ship"}`)
	writeFile(t, filepath.Join(src, "node_modules", "dep", "index.js"), "secret dependency")
	writeFile(t, filepath.Join(outside, "secret.ts"), "outside")
	writeFile(t, filepath.Join(outside, "leak.ts"), "outside dir")

	if err := os.Symlink(filepath.Join(outside, "secret.ts"), filepath.Join(src, "link.ts")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "linked")); err != nil {
		t.Fatal(err)
	}

	payload, err := ZipDir(src)
	if err != nil {
		t.Fatalf("ZipDir: %v", err)
	}
	entries := zipEntries(t, payload)
	for _, want := range []string{"bot.ts", "codearena.yaml", "package.json", "README.md", "src/helper.ts"} {
		if !entries[want] {
			t.Errorf("missing allowed file %q in %v", want, entries)
		}
	}
	for _, unwanted := range []string{
		".env", "credentials.json", "server.key", ".codearena/config.json",
		"node_modules/dep/index.js", "link.ts", "linked/secret.ts", "linked/leak.ts",
	} {
		if entries[unwanted] {
			t.Errorf("unexpected file in payload: %q", unwanted)
		}
	}
}

func TestZipDirRejectsOversizedFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "bot")
	writeFile(t, filepath.Join(src, "huge.ts"), strings.Repeat("a", MaxFileBytes+1))
	if _, err := ZipDir(src); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("error = %v, want size limit error", err)
	}
}

func TestZipDirRejectsSymlinkRootAndInvalidSource(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "bot")
	writeFile(t, filepath.Join(src, "bot.ts"), "ok")
	link := filepath.Join(root, "link")
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ZipDir(link); err == nil {
		t.Fatal("symlink root accepted")
	}
	file := filepath.Join(root, "file.ts")
	writeFile(t, file, "ok")
	if _, err := ZipDir(file); err == nil {
		t.Fatal("regular file accepted as source directory")
	}
	if _, err := ZipDir(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestAllowedSourceFile(t *testing.T) {
	allowed := []string{"bot.ts", "codearena.yaml", "package.json", "README.md", "Main.java"}
	for _, name := range allowed {
		if !allowedSourceFile(name) {
			t.Errorf("allowedSourceFile(%q) = false", name)
		}
	}
	denied := []string{".env", ".env.local", "id_rsa", ".npmrc", "aws-credentials.json", "api-secret.ts", "token.txt", "dump.pem"}
	for _, name := range denied {
		if allowedSourceFile(name) {
			t.Errorf("allowedSourceFile(%q) = true, want false", name)
		}
	}
}
