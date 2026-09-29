package declscope_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSkillFrontmatterParses reads the frontmatter of every skill as YAML. An
// unquoted description holding ": ", as in "//declscope: directive", is a
// mapping key to YAML, and the skill then fails to load or render.
func TestSkillFrontmatterParses(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("skills", "*", "SKILL.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no skills found: %v", err)
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(string(body), "---\n", 3)
		if len(parts) != 3 || parts[0] != "" {
			t.Errorf("%s: no frontmatter", path)
			continue
		}
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
			t.Errorf("%s: frontmatter is not YAML: %v", path, err)
			continue
		}
		if dir := filepath.Base(filepath.Dir(path)); meta.Name != dir {
			t.Errorf("%s: name is %q, want the directory name %q", path, meta.Name, dir)
		}
		if meta.Description == "" {
			t.Errorf("%s: no description", path)
		}
	}
}
