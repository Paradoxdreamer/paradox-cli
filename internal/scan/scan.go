package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// Result holds everything discovered by a scan.
type Result struct {
	Root           string
	HasParadoxYAML bool
	ParadoxYAML    string
	ProjectName    string
	Services       []string
	Dockerfiles    []string
	ComposeFiles   []string
	Runtimes       []Runtime
	Missing        []string // human-readable suggestions
}

// Runtime describes a detected language/runtime.
type Runtime struct {
	Name    string // e.g. "Go", "Node.js", "Python"
	Marker  string // e.g. "go.mod", "package.json"
	Version string // best-effort, often empty
}

// Scan walks dir (and immediate children) looking for project artifacts.
func Scan(dir string) (*Result, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	r := &Result{Root: abs}

	// 1. paradox.yaml
	for _, name := range []string{"paradox.yaml", "paradox.yml", filepath.Join(".paradox", "paradox.yaml")} {
		p := filepath.Join(abs, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			r.HasParadoxYAML = true
			r.ParadoxYAML = p
			// Best-effort parse of project_name and services
			if data, err := os.ReadFile(p); err == nil {
				r.ProjectName, r.Services = parseParadoxYAML(string(data))
			}
			break
		}
	}

	// 2. Dockerfiles (root + one level deep)
	r.Dockerfiles = findFiles(abs, []string{"Dockerfile", "Dockerfile.*"}, 1)

	// 3. Compose files
	r.ComposeFiles = findFiles(abs, []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	}, 0)

	// 4. Language runtimes
	r.Runtimes = detectRuntimes(abs)

	// 5. Suggestions
	if !r.HasParadoxYAML {
		r.Missing = append(r.Missing, "No paradox.yaml found — run `paradox init`")
	}
	if len(r.Dockerfiles) == 0 && len(r.ComposeFiles) == 0 && len(r.Runtimes) == 0 {
		r.Missing = append(r.Missing, "No Dockerfiles, compose files, or language markers detected")
	}

	return r, nil
}

func parseParadoxYAML(content string) (projectName string, services []string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "project_name:") {
			projectName = strings.TrimSpace(strings.TrimPrefix(line, "project_name:"))
			projectName = strings.Trim(projectName, `"'`)
		}
		// Very naive services list: look for "- name" under services:
		if strings.HasPrefix(line, "- ") && !strings.Contains(line, ":") {
			svc := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if svc != "" {
				services = append(services, svc)
			}
		}
	}
	return
}

func findFiles(root string, patterns []string, maxDepth int) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if d.IsDir() {
			if depth > maxDepth {
				return filepath.SkipDir
			}
			// skip common noise
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "bin" || name == ".paradox" {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		for _, pat := range patterns {
			matched, _ := filepath.Match(pat, base)
			if matched {
				found = append(found, path)
				break
			}
		}
		return nil
	})
	return found
}

func detectRuntimes(root string) []Runtime {
	markers := []struct {
		file string
		name string
	}{
		{"go.mod", "Go"},
		{"package.json", "Node.js"},
		{"requirements.txt", "Python"},
		{"pyproject.toml", "Python"},
		{"Cargo.toml", "Rust"},
		{"pom.xml", "Java (Maven)"},
		{"build.gradle", "Java (Gradle)"},
		{"build.gradle.kts", "Java (Gradle)"},
		{"Gemfile", "Ruby"},
		{"composer.json", "PHP"},
		{"mix.exs", "Elixir"},
		{"pubspec.yaml", "Dart/Flutter"},
	}

	var runtimes []Runtime
	seen := map[string]bool{}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" || name == "bin" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		base := filepath.Base(path)
		for _, m := range markers {
			if base == m.file && !seen[m.name] {
				seen[m.name] = true
				runtimes = append(runtimes, Runtime{Name: m.name, Marker: path})
			}
		}
		return nil
	})
	return runtimes
}
