package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateName(t *testing.T) {
	valid := []string{"my-bot", "Bot_1", "robot.2", "a"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "../outside", "a/b", `a\b`, "/tmp/robot", ".", "..", ".hidden", "bad name", strings.Repeat("a", 65), "CON"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) unexpectedly succeeded", name)
		}
	}
}

func TestValidateLanguage(t *testing.T) {
	if err := ValidateLanguage("typescript"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLanguage(" Python "); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLanguage("java"); err == nil {
		t.Fatal("unsupported language accepted")
	}
}

func TestInitProjectCreatesCompleteProject(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := InitProject("my-bot", "typescript"); err != nil {
		t.Fatalf("InitProject: %v", err)
	}

	manifestData, err := os.ReadFile(filepath.Join("my-bot", "codearena.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "my-bot" || manifest.Language != "typescript" || manifest.Version != "1.0.0" || manifest.ID == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	pkgData, err := os.ReadFile(filepath.Join("my-bot", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(pkgData, &pkg); err != nil {
		t.Fatalf("package.json is not valid JSON: %v", err)
	}
	if pkg["name"] != "my-bot" {
		t.Fatalf("package name = %v", pkg["name"])
	}
	botData, err := os.ReadFile(filepath.Join("my-bot", "bot.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(botData), `BOT_NAME: string = "my-bot"`) {
		t.Fatalf("bot.ts does not contain the escaped bot name: %s", botData)
	}
}

func TestInitProjectPython(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := InitProject("python-bot", "Python"); err != nil {
		t.Fatalf("InitProject: %v", err)
	}
	for _, file := range []string{"codearena.yaml", "bot.py"} {
		if _, err := os.Stat(filepath.Join("python-bot", file)); err != nil {
			t.Errorf("missing %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join("python-bot", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected package.json for python: %v", err)
	}
}

func TestInitProjectDoesNotCreateAnythingForBadInput(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	for _, tc := range []struct{ name, lang string }{
		{"../escape", "typescript"},
		{"valid-name", "java"},
	} {
		if err := InitProject(tc.name, tc.lang); err == nil {
			t.Fatalf("InitProject(%q, %q) unexpectedly succeeded", tc.name, tc.lang)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("leftover files after failed init: %v", entries)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape")); err == nil {
		t.Fatal("path traversal created a directory outside the target")
	}
}

func TestTemplateValuesAreEscaped(t *testing.T) {
	const hostile = `quote"backslash\and<tag>`
	data, err := templateData(hostile)
	if err != nil {
		t.Fatal(err)
	}
	var unquoted string
	if err := json.Unmarshal([]byte(data.BotNameTS), &unquoted); err != nil || unquoted != hostile {
		t.Fatalf("escaped literal = %s, err = %v", data.BotNameTS, err)
	}
	if data.BotNameJSON != data.BotNameTS {
		t.Fatal("JSON and TS literals differ")
	}

	dest := filepath.Join(t.TempDir(), "package.json")
	if err := renderTemplate("typescript/package.json.tmpl", dest, data); err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(content, &pkg); err != nil {
		t.Fatalf("rendered package.json is not valid JSON: %v\n%s", err, content)
	}
	if pkg["name"] != hostile {
		t.Fatalf("name = %v, want %q", pkg["name"], hostile)
	}
}

func TestRenderTemplatePropagatesErrors(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "file")
	if err := renderTemplate("does-not-exist.tmpl", dest, TemplateData{}); err == nil {
		t.Fatal("missing template did not return an error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("failed template still created a file: %v", err)
	}
}

func TestClassNameIsAValidIdentifier(t *testing.T) {
	tests := map[string]string{
		"my-bot":  "MyBot",
		"1-bot":   "Bot1Bot",
		"robot_2": "Robot2",
	}
	for input, want := range tests {
		if got := className(input); got != want {
			t.Errorf("className(%q) = %q, want %q", input, got, want)
		}
	}
}
