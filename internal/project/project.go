package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/codearena-platform/codearena-cli/internal/templates"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// TemplateData holds values rendered into the embedded project templates.
// Strings destined for source code are pre-quoted with JSON escaping, so a
// project name can never break out of its literal.
type TemplateData struct {
	BotName      string
	BotClassName string
	BotNameJSON  string // JSON/Python/TS string literal, including quotes
	BotNameTS    string // TypeScript string literal, including quotes
}

type Manifest struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Language string `yaml:"language"`
}

var (
	validName  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	identifier = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateName rejects traversal, absolute paths, empty and unsafe names.
func ValidateName(name string) error {
	if name == "" || name != strings.TrimSpace(name) {
		return fmt.Errorf("nome de robô inválido: %q", name)
	}
	if filepath.IsAbs(name) || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return fmt.Errorf("nome de robô inválido (use apenas um nome de diretório): %q", name)
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("nome de robô inválido (use letras, números, '.', '_' ou '-'; máximo 64 caracteres): %q", name)
	}
	if reservedNames[strings.ToLower(strings.SplitN(name, ".", 2)[0])] {
		return fmt.Errorf("nome de robô reservado: %q", name)
	}
	return nil
}

// ValidateLanguage rejects languages without a bundled template. It runs before
// any filesystem mutation so an unsupported language cannot leave a manifest.
func ValidateLanguage(lang string) error {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "typescript", "python":
		return nil
	default:
		return fmt.Errorf("linguagem não suportada no momento: %s", lang)
	}
}

// InitProject sets up a new bot directory with a manifest and templates. On any
// failure the partially created directory is removed.
func InitProject(name, lang string) (err error) {
	name = strings.TrimSpace(name)
	lang = strings.ToLower(strings.TrimSpace(lang))
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := ValidateLanguage(lang); err != nil {
		return err
	}

	if err := os.Mkdir(name, 0755); err != nil {
		return fmt.Errorf("falha ao criar o diretório: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(name)
		}
	}()

	data, err := templateData(name)
	if err != nil {
		return err
	}

	manifest := Manifest{
		ID:       uuid.NewString(),
		Name:     name,
		Version:  "1.0.0",
		Language: lang,
	}
	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("falha ao criar arquivo de manifesto: %w", err)
	}
	if err := os.WriteFile(filepath.Join(name, "codearena.yaml"), manifestData, 0644); err != nil {
		return fmt.Errorf("falha ao salvar o manifesto: %w", err)
	}

	var files []struct{ tmpl, dest string }
	switch lang {
	case "typescript":
		files = []struct{ tmpl, dest string }{
			{"typescript/package.json.tmpl", "package.json"},
			{"typescript/bot.ts.tmpl", "bot.ts"},
		}
	case "python":
		files = []struct{ tmpl, dest string }{{"python/bot.py.tmpl", "bot.py"}}
	}
	for _, file := range files {
		if err := renderTemplate(file.tmpl, filepath.Join(name, file.dest), data); err != nil {
			return fmt.Errorf("falha ao gerar %s: %w", file.dest, err)
		}
	}
	return nil
}

func templateData(name string) (TemplateData, error) {
	quoted, err := json.Marshal(name)
	if err != nil {
		return TemplateData{}, fmt.Errorf("escapar nome do robô: %w", err)
	}
	literal := string(quoted)
	return TemplateData{
		BotName:      name,
		BotClassName: className(name),
		BotNameJSON:  literal,
		BotNameTS:    literal,
	}, nil
}

// className derives a valid identifier from a validated project name.
func className(name string) string {
	parts := identifier.Split(name, -1)
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		b.WriteString(strings.ToUpper(string(runes[0])))
		b.WriteString(string(runes[1:]))
	}
	class := b.String()
	if class == "" {
		class = "MyBot"
	}
	if !unicode.IsLetter(rune(class[0])) {
		class = "Bot" + class
	}
	return class
}

func renderTemplate(tmplPath, destPath string, data interface{}) error {
	tmplContent, err := templates.FS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("ler template %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Option("missingkey=error").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parsear template %s: %w", tmplPath, err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("criar arquivo %s: %w", destPath, err)
	}
	execErr := t.Execute(f, data)
	closeErr := f.Close()
	if execErr != nil {
		return fmt.Errorf("executar template %s: %w", tmplPath, execErr)
	}
	if closeErr != nil {
		return fmt.Errorf("fechar arquivo %s: %w", destPath, closeErr)
	}
	return nil
}
