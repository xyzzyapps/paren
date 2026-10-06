package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Front is the optional YAML block at the top of an HSL document.
//
//	---
//	interpreter: python
//	imports:
//	  - twice.hsl
//	---
type Front struct {
	Interpreter string         `yaml:"interpreter"`
	Suffix      string         `yaml:"suffix"`
	Imports     []string       `yaml:"imports"`
	Attrs       map[string]any `yaml:"-"`
}

func splitFront(source string) (Front, string, error) {
	var front Front
	if !strings.HasPrefix(source, "---\n") && !strings.HasPrefix(source, "---\r\n") {
		return front, source, nil
	}
	rest := source[4:]
	if strings.HasPrefix(source, "---\r\n") {
		rest = source[5:]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return front, "", fmt.Errorf("unclosed YAML frontmatter")
	}
	raw := rest[:end]
	body := rest[end+4:]
	if strings.HasPrefix(body, "\r\n") {
		body = body[2:]
	} else if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}
	if err := yaml.Unmarshal([]byte(raw), &front); err != nil {
		return front, "", fmt.Errorf("frontmatter: %w", err)
	}
	var attrs map[string]any
	if err := yaml.Unmarshal([]byte(raw), &attrs); err != nil {
		return front, "", fmt.Errorf("frontmatter: %w", err)
	}
	front.Attrs = attrs
	if front.Interpreter == "" {
		front.Interpreter = "python"
	}
	if front.Suffix == "" {
		front.Suffix = suffixFor(front.Interpreter)
	}
	if !strings.HasPrefix(front.Suffix, ".") {
		front.Suffix = "." + front.Suffix
	}
	return front, body, nil
}

func suffixFor(interpreter string) string {
	switch interpreter {
	case "python", "python3":
		return ".py"
	case "node", "nodejs", "deno", "bun":
		return ".js"
	case "ruby":
		return ".rb"
	case "perl":
		return ".pl"
	case "php":
		return ".php"
	case "lua":
		return ".lua"
	case "go":
		return ".go"
	case "gcc", "clang", "cc", "tcc":
		return ".c"
	case "g++", "clang++":
		return ".cpp"
	case "sbcl", "clisp", "ecl":
		return ".lisp"
	default:
		return ".txt"
	}
}
