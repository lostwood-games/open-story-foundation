package yamlio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// EncodeJSON writes a resource tree as JSON with two-space indentation, keys
// in the order the tree has them, and a trailing newline. encoding/json is not
// used for whole documents because Go maps would lose that order.
func EncodeJSON(n *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, n, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeJSON(buf *bytes.Buffer, n *yaml.Node, depth int) error {
	indent := strings.Repeat("  ", depth)
	switch n.Kind {
	case yaml.DocumentNode:
		return writeJSON(buf, n.Content[0], depth)
	case yaml.MappingNode:
		if len(n.Content) == 0 {
			buf.WriteString("{}")
			return nil
		}
		buf.WriteString("{\n")
		for i := 0; i+1 < len(n.Content); i += 2 {
			buf.WriteString(indent + "  ")
			if err := writeScalar(buf, n.Content[i].Value); err != nil {
				return err
			}
			buf.WriteString(": ")
			if err := writeJSON(buf, n.Content[i+1], depth+1); err != nil {
				return err
			}
			if i+2 < len(n.Content) {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(indent + "}")
	case yaml.SequenceNode:
		if len(n.Content) == 0 {
			buf.WriteString("[]")
			return nil
		}
		buf.WriteString("[\n")
		for i, item := range n.Content {
			buf.WriteString(indent + "  ")
			if err := writeJSON(buf, item, depth+1); err != nil {
				return err
			}
			if i+1 < len(n.Content) {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(indent + "]")
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!bool", "!!int", "!!null":
			buf.WriteString(n.Value)
		default:
			return writeScalar(buf, n.Value)
		}
	default:
		return fmt.Errorf("cannot write a %v node as JSON", n.Kind)
	}
	return nil
}

// writeScalar writes s as a JSON string without escaping <, > and &.
func writeScalar(buf *bytes.Buffer, s string) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // the encoder's newline
	return nil
}
