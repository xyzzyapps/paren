package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguages(t *testing.T) {
	root := moduleRoot(t)
	cases := []struct {
		file string
		bin  string
		want string
	}{
		{"python.hsl", "python", "hello from python\n"},
		{"node.hsl", "node", "hello from node\n"},
		{"go.hsl", "go", "hello from go\n"},
		{"c.hsl", "gcc", "hello from c\n"},
		{"cpp.hsl", "g++", "hello from c++\n"},
		{"lisp.hsl", "sbcl", "hello from lisp\n"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			if _, err := exec.LookPath(tc.bin); err != nil {
				t.Skip(tc.bin + " is not on PATH")
			}
			cmd := exec.Command("go", "run", ".", "run", filepath.Join("examples", "langs", tc.file))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			got := strings.ReplaceAll(string(out), "\r\n", "\n")
			if got != tc.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestBracketConfig(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, "examples", "brackets", "hello.hsl")
	script, err := Transpile(path, mustRules(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if script.Source != "print(\"hello from brackets\")\n" {
		t.Fatalf("config delims got %q", script.Source)
	}

	cmd := exec.Command("go", "run", ".", "transpile", "--delims", "[]", path)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if string(out) != "print(\"hello from brackets\")\n" {
		t.Fatalf("cli delims got %q", out)
	}
}

func mustRules(t *testing.T, path string) Rules {
	t.Helper()
	rules, err := resolveRules(path, "", cliOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func TestCLIDelimsOverrideConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hsl.toml"), []byte("delimiters = [\"[]\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "note.hsl")
	body := "---\ninterpreter: python\n---\n(print(1))\n"
	if err := os.WriteFile(doc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := resolveRules(doc, "python", cliOpts{delims: []string{"()"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Extract("(print(1))", rules)
	if err != nil {
		t.Fatal(err)
	}
	if got != "print(1)\n" {
		t.Fatalf("override got %q", got)
	}
}
