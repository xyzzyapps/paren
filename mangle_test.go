package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMangleRewritesFile(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.hsl")
	if err := os.WriteFile(other, []byte("(imported = 1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "note.hsl")
	text := `---
interpreter: python
note: keep
imports:
  - other.hsl
---
The title is <#title hello>.
Items are <.item one> and <.item two>.
(print(1))
<:
return [ids.title, classes.item[1], parens[3], imports[0], attrs.note].join("\n");
>
`
	if err := os.WriteFile(doc, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	script, err := Transpile(doc, DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	want := "imported = 1\n<#title hello>\n<.item two>\nprint(1)\nother.hsl\nkeep\n"
	if script.Source != want {
		t.Fatalf("got %q", script.Source)
	}
}

func TestSilentPrintsNothing(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "note.hsl")
	text := "(print(1))\n<:\nreturn \"print(2)\\n\";\n>\n"
	if err := os.WriteFile(doc, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, "transpile", "--silent", doc)
	if code != 0 {
		t.Fatalf("silent exit %d", code)
	}
	if out != "" {
		t.Fatalf("silent wrote %q", out)
	}
	out, code = captureStdout(t, doc)
	if code != 0 || out != "print(2)\n" {
		t.Fatalf("default exit %d output %q", code, out)
	}
}

func captureStdout(t *testing.T, args ...string) (string, int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := run(args)
	w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return string(buf[:n]), code
}

func TestMangleOnce(t *testing.T) {
	_, _, err := stripMangle("<: 1 > and <: 2 >", DefaultRules())
	if err == nil {
		t.Fatal("expected a second block to fail")
	}
}

func TestMangleStringHidesCloser(t *testing.T) {
	body, js, err := stripMangle("<: return \"a > b\"; > (print(1))", DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if js == nil || !strings.Contains(*js, `a > b`) {
		t.Fatalf("js %v", js)
	}
	if !strings.Contains(body, "(print(1))") {
		t.Fatal(body)
	}
}
