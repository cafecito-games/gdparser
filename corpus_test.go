package gdparser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/projectconfig"
	projectast "github.com/cafecito-games/gdparser/projectconfig/ast"
	"github.com/cafecito-games/gdparser/shader"
	shaderast "github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/textresource"
)

// TestCorpus is an opt-in integration test for external Godot projects. Set
// GDPARSER_CORPUS to a project tree containing supported files to enable it.
func TestCorpus(t *testing.T) {
	root := os.Getenv("GDPARSER_CORPUS")
	if root == "" {
		t.Skip("set GDPARSER_CORPUS to run corpus validation")
	}
	counts := map[string]int{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		isProjectConfig := filepath.Base(path) == "project.godot"
		if extension != ".gd" && extension != ".tscn" && extension != ".tres" && extension != ".escn" &&
			extension != ".gdshader" && extension != ".gdshaderinc" && !isProjectConfig {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var kind string
		var firstValue, secondValue any
		switch {
		case extension == ".gd":
			kind = "GDScript"
			first, parseErr := gdparser.ParseFile(path, source)
			if parseErr != nil {
				t.Errorf("parse %s: %v", path, parseErr)
				return nil
			}
			second, parseErr := gdparser.ParseFile(path, []byte(gdparser.Format(first)))
			if parseErr != nil {
				t.Errorf("reparse %s: %v", path, parseErr)
				return nil
			}
			firstValue = normalizedJSON(first, true)
			secondValue = normalizedJSON(second, true)
		case extension == ".tscn", extension == ".tres", extension == ".escn":
			kind = "text resource"
			first, parseErr := textresource.ParseFile(path, source)
			if parseErr != nil {
				t.Errorf("parse %s: %v", path, parseErr)
				return nil
			}
			second, parseErr := textresource.ParseFile(path, []byte(textresource.Format(first)))
			if parseErr != nil {
				t.Errorf("reparse %s: %v", path, parseErr)
				return nil
			}
			firstValue = normalizeValue(textresource.JSONValue(first), true)
			secondValue = normalizeValue(textresource.JSONValue(second), true)
		case isProjectConfig:
			kind = "project config"
			first, parseErr := projectconfig.ParseFile(path, source)
			if parseErr != nil {
				t.Errorf("parse %s: %v", path, parseErr)
				return nil
			}
			second, parseErr := projectconfig.ParseFile(path, []byte(projectconfig.Format(first)))
			if parseErr != nil {
				t.Errorf("reparse %s: %v", path, parseErr)
				return nil
			}
			firstValue = normalizeValue(projectast.JSONValue(first), true)
			secondValue = normalizeValue(projectast.JSONValue(second), true)
		case extension == ".gdshader", extension == ".gdshaderinc":
			kind = "shader"
			first, parseErr := shader.ParseFile(path, source)
			if parseErr != nil {
				t.Errorf("parse %s: %v", path, parseErr)
				return nil
			}
			second, parseErr := shader.ParseFile(path, []byte(shader.Format(first)))
			if parseErr != nil {
				t.Errorf("reparse %s: %v", path, parseErr)
				return nil
			}
			firstValue = normalizeValue(shaderast.JSONValue(first), true)
			secondValue = normalizeValue(shaderast.JSONValue(second), true)
		default:
			return nil
		}
		counts[kind]++
		if !reflect.DeepEqual(firstValue, secondValue) {
			t.Errorf("structural round trip changed %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) == 0 {
		t.Fatal(fmt.Errorf("no supported files found under %s", root))
	}
	for kind, count := range counts {
		t.Logf("validated %d %s files", count, kind)
	}
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
