package uidfile

// JSONValue converts an AST to a JSON-friendly value with a kind discriminator
// on every node.
func JSONValue(node Node) any {
	switch node := node.(type) {
	case nil:
		return nil
	case *File:
		value := map[string]any{
			"kind": "File",
			"span": node.SourceSpan,
			"uid":  JSONValue(node.UID),
		}
		if node.Name != "" {
			value["name"] = node.Name
		}
		return value
	case *UID:
		return map[string]any{
			"kind":  "UID",
			"span":  node.SourceSpan,
			"value": node.Value,
		}
	default:
		return nil
	}
}
