package gdparser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
)

// TestCorpus is an opt-in integration test for large external GDScript trees.
// Set GDPARSER_CORPUS to a directory containing .gd files to enable it.
func TestCorpus(t *testing.T) {
	root := os.Getenv("GDPARSER_CORPUS")
	if root == "" {
		t.Skip("set GDPARSER_CORPUS to run corpus validation")
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".gd" {
			return nil
		}
		count++
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		first, parseErr := gdparser.ParseFile(path, source)
		if parseErr != nil {
			t.Errorf("parse %s: %v", path, parseErr)
			return nil
		}
		formatted := gdparser.Format(first)
		second, parseErr := gdparser.ParseFile(path, []byte(formatted))
		if parseErr != nil {
			t.Errorf("reparse %s: %v", path, parseErr)
			return nil
		}
		firstValue := normalizedJSON(first, true)
		secondValue := normalizedJSON(second, true)
		if !reflect.DeepEqual(firstValue, secondValue) {
			t.Errorf("structural round trip changed %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal(fmt.Errorf("no .gd files found under %s", root))
	}
	t.Logf("validated %d GDScript files", count)
}

func normalizedJSON(node ast.Node, root bool) any {
	return normalizeValue(ast.JSONValue(node), root)
}

func normalizeValue(value any, root bool) any {
	switch current := value.(type) {
	case map[string]any:
		delete(current, "span")
		if root {
			delete(current, "name")
		}
		for key, child := range current {
			current[key] = normalizeValue(child, false)
		}
		return current
	case []any:
		for index, child := range current {
			current[index] = normalizeValue(child, false)
		}
		return current
	default:
		return current
	}
}
