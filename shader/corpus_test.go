package shader_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/shader"
)

func TestCorpus(t *testing.T) {
	root := os.Getenv("GDSHADER_CORPUS")
	if root == "" {
		t.Skip("set GDSHADER_CORPUS to validate an external shader tree")
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !(strings.HasSuffix(path, ".gdshader") || strings.HasSuffix(path, ".gdshaderinc")) {
			return nil
		}
		t.Run(strings.TrimPrefix(path, root), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := shader.ParseFile(path, source)
			if err != nil {
				t.Fatal(err)
			}
			formatted := shader.Format(file)
			reparsed, err := shader.ParseFile(path, []byte(formatted))
			if err != nil {
				t.Fatalf("formatted shader did not parse: %v\n%s", err, formatted)
			}
			if again := shader.Format(reparsed); again != formatted {
				t.Fatalf("formatter is not idempotent")
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
