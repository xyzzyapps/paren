package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Script is one transpiled document. Interpreter and Suffix come from the
// root file. Imports are expanded before the root body.
type Script struct {
	Source      string
	Interpreter string
	Suffix      string
}

// Transpile reads path and concatenates imported HSL documents, then the
// code spans of path itself.
func Transpile(path string, rules Rules) (Script, error) {
	seen := map[string]struct{}{}
	var b strings.Builder
	front, err := includeFile(path, seen, &b, rules)
	if err != nil {
		return Script{}, err
	}
	interpreter := front.Interpreter
	if interpreter == "" {
		interpreter = "python"
	}
	suffix := front.Suffix
	if suffix == "" {
		suffix = suffixFor(interpreter)
	}
	return Script{Source: b.String(), Interpreter: interpreter, Suffix: suffix}, nil
}

func includeFile(path string, seen map[string]struct{}, out *strings.Builder, rules Rules) (Front, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Front{}, err
	}
	abs = filepath.Clean(abs)
	if _, ok := seen[abs]; ok {
		return Front{}, fmt.Errorf("import cycle at %s", abs)
	}
	seen[abs] = struct{}{}

	data, err := os.ReadFile(abs)
	if err != nil {
		return Front{}, err
	}
	front, body, err := splitFront(string(data))
	if err != nil {
		return Front{}, fmt.Errorf("%s: %w", abs, err)
	}
	rules.Markdown = isMarkdown(abs)
	dir := filepath.Dir(abs)
	for _, imp := range front.Imports {
		impPath := imp
		if !filepath.IsAbs(impPath) {
			impPath = filepath.Join(dir, impPath)
		}
		if _, err := includeFile(impPath, seen, out, rules); err != nil {
			return Front{}, err
		}
	}
	stripped, js, err := stripMangle(body, rules)
	if err != nil {
		return Front{}, fmt.Errorf("%s: %w", abs, err)
	}
	script, err := Extract(stripped, rules)
	if err != nil {
		return Front{}, fmt.Errorf("%s: %w", abs, err)
	}
	if js != nil {
		script, err = runMangle(*js, stripped, front, rules)
		if err != nil {
			return Front{}, fmt.Errorf("%s: %w", abs, err)
		}
	}
	out.WriteString(script)
	return front, nil
}
