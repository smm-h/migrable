package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tomledit "github.com/smm-h/go-toml-edit"
)

type Config struct {
	MigrationsDir string
	VersionFile   string
	Files         map[string]string
	BaseDir       string
}

const configFileName = "migrable.toml"

func Load(configDir string) (*Config, error) {
	path, err := findConfig(configDir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}
	return parse(data, filepath.Dir(absPath))
}

func findConfig(configDir string) (string, error) {
	if configDir != "" {
		path := filepath.Join(configDir, configFileName)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("config not found: %s does not exist", path)
		}
		return path, nil
	}

	dotPath := filepath.Join(".", configFileName)
	subPath := filepath.Join(".migrable", configFileName)

	dotExists := fileExists(dotPath)
	subExists := fileExists(subPath)

	if dotExists && subExists {
		return "", fmt.Errorf("ambiguous config: found %s in both . and .migrable/", configFileName)
	}
	if dotExists {
		return dotPath, nil
	}
	if subExists {
		return subPath, nil
	}
	return "", fmt.Errorf("config not found: %s not found in . or .migrable/", configFileName)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parse(data []byte, baseDir string) (*Config, error) {
	doc, err := tomledit.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse migrable.toml: %w", err)
	}

	migrationsDir, ok := doc.GetString("migrations_dir")
	if !ok {
		return nil, fmt.Errorf("migrable.toml: missing required key \"migrations_dir\"")
	}

	files, err := extractFiles(doc)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		MigrationsDir: migrationsDir,
		Files:         files,
		BaseDir:       baseDir,
	}

	if len(files) > 1 {
		vf, ok := doc.GetString("version_file")
		if !ok {
			return nil, fmt.Errorf("migrable.toml: \"version_file\" is required when [files] has more than one entry")
		}
		if _, exists := files[vf]; !exists {
			return nil, fmt.Errorf("migrable.toml: \"version_file\" value %q is not a key in [files]", vf)
		}
		cfg.VersionFile = vf
	}

	for key, path := range cfg.Files {
		expanded, err := expandEnvStrict(path)
		if err != nil {
			return nil, fmt.Errorf("migrable.toml: [files].%s: %w", key, err)
		}
		cfg.Files[key] = expanded
	}

	return cfg, nil
}

func extractFiles(doc *tomledit.DocumentNode) (map[string]string, error) {
	filesNode := doc.Get("files")
	if filesNode == nil {
		return nil, fmt.Errorf("migrable.toml: missing required section [files]")
	}

	var children []tomledit.Node
	switch n := filesNode.(type) {
	case *tomledit.TableNode:
		children = n.Children
	case *tomledit.InlineTableNode:
		children = n.Children
	default:
		return nil, fmt.Errorf("migrable.toml: [files] must be a table, got %s", filesNode.Type())
	}

	if len(children) == 0 {
		return nil, fmt.Errorf("migrable.toml: [files] must have at least one entry")
	}

	files := make(map[string]string, len(children))
	for _, child := range children {
		kv, ok := child.(*tomledit.KeyValueNode)
		if !ok {
			continue
		}
		key := kv.Key.Parts[0]
		strNode, ok := kv.Val.(*tomledit.StringNode)
		if !ok {
			return nil, fmt.Errorf("migrable.toml: [files].%s must be a string", key)
		}
		files[key] = strNode.Val
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("migrable.toml: [files] must have at least one entry")
	}

	return files, nil
}

// expandEnvStrict expands $VAR and ${VAR} in s, returning an error if any
// referenced variable is not set in the environment.
func expandEnvStrict(s string) (string, error) {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '$' {
			result.WriteByte(s[i])
			i++
			continue
		}
		i++ // skip '$'
		if i >= len(s) {
			result.WriteByte('$')
			break
		}

		var name string
		if s[i] == '{' {
			i++ // skip '{'
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("unterminated ${...} in %q", s)
			}
			name = s[i : i+end]
			i += end + 1 // skip past '}'
		} else {
			start := i
			for i < len(s) && isVarChar(s[i]) {
				i++
			}
			name = s[start:i]
		}

		if name == "" {
			return "", fmt.Errorf("empty variable name in %q", s)
		}
		val, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("environment variable %q is not set", name)
		}
		result.WriteString(val)
	}
	return result.String(), nil
}

func isVarChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
