package configfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/configfile"
)

// TestCorpus validates ConfigFile documents under the optional, read-only
// GDPARSER_CONFIGFILE_CORPUS path. It is intentionally absent from the
// default test requirements.
func TestCorpus(t *testing.T) {
	root := os.Getenv("GDPARSER_CONFIGFILE_CORPUS")
	if root == "" {
		t.Skip("GDPARSER_CONFIGFILE_CORPUS is not set")
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
		if !entry.IsDir() && isConfigFile(path) {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no ConfigFile documents found under %s", root)
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := configfile.ParseFile(path, source)
			if err != nil {
				t.Fatal(err)
			}
			formatted := configfile.Format(file)
			if _, err := configfile.ParseFile(path, []byte(formatted)); err != nil {
				t.Fatalf("reparse canonical output: %v", err)
			}
		})
	}
}

func isConfigFile(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return filepath.Base(path) == "project.godot" || extension == ".cfg" || extension == ".gdextension" || extension == ".import" || extension == ".remap"
}
