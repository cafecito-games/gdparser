package projectconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cafecito-games/gdparser/projectconfig"
)

// TestCorpus validates project.godot files under the optional, read-only
// GDPARSER_PROJECTCONFIG_CORPUS path. It is intentionally absent from the
// default test requirements.
func TestCorpus(t *testing.T) {
	root := os.Getenv("GDPARSER_PROJECTCONFIG_CORPUS")
	if root == "" {
		t.Skip("GDPARSER_PROJECTCONFIG_CORPUS is not set")
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	if !info.IsDir() {
		paths = append(paths, root)
	} else if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && (entry.Name() == ".git" || entry.Name() == ".godot") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == "project.godot" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no project.godot files found under %s", root)
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := projectconfig.ParseFile(path, source)
			if err != nil {
				t.Fatal(err)
			}
			formatted := projectconfig.Format(file)
			if _, err := projectconfig.ParseFile(path, []byte(formatted)); err != nil {
				t.Fatalf("reparse canonical output: %v", err)
			}
		})
	}
}
