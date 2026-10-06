package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
		bad  bool
	}{
		{"order", "Say (print(\"a\")) then <print(\"b\")> and (print(\"c\")).", "print(\"a\")\nprint(\"b\")\nprint(\"c\")\n", false},
		{"nest", "Compute (print(1 + (2 * 3))).", "print(1 + (2 * 3))\n", false},
		{"angles in parens", "(print(3 < 4))", "print(3 < 4)\n", false},
		{"string", "(print(\")\"))", "print(\")\")\n", false},
		{"comment", "(x = 1  # ) still inside\n)", "x = 1  # ) still inside\n", false},
		{"escape", "Mention \\(parens\\) then (x = 1).", "x = 1\n", false},
		{"block", "(\ndef f():\n    return 1\n)\n", "def f():\n    return 1\n", false},
		{"empty", "no code here", "", false},
		{"unclosed", "start (print(1)", "", true},
		{"stray", "done ) too soon", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract(tc.in, DefaultRules())
			if tc.bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.out {
				t.Fatalf("got %q want %q", got, tc.out)
			}
		})
	}
}

func TestImportsAndRun(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "twice.hsl")
	doc := filepath.Join(dir, "greet.hsl")
	if err := os.WriteFile(lib, []byte("(\ndef twice(message):\n    print(message)\n    print(message)\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := []byte("---\ninterpreter: python\nimports:\n  - twice.hsl\n---\nStore it (greeting = \"hi\").\nUse it (twice(greeting)).\n")
	if err := os.WriteFile(doc, body, 0o644); err != nil {
		t.Fatal(err)
	}
	script, err := Transpile(doc, DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	want := "def twice(message):\n    print(message)\n    print(message)\ngreeting = \"hi\"\ntwice(greeting)\n"
	if script.Source != want {
		t.Fatalf("got %q", script.Source)
	}
	if script.Interpreter != "python" {
		t.Fatal(script.Interpreter)
	}
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("python is not on PATH")
	}
	cmd := exec.Command("go", "run", ".", "run", doc)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != "hi\nhi\n" {
		t.Fatalf("run output %q", out)
	}
}

func TestMissingInterpreter(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "nope.hsl")
	text := "---\ninterpreter: hsl-missing-interp\nsuffix: .txt\n---\n(print(1))\n"
	if err := os.WriteFile(doc, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "run", doc)
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected failure")
	}
	if script, err := Transpile(doc, DefaultRules()); err != nil || script.Source != "print(1)\n" {
		t.Fatalf("transpile still works: %v %q", err, script.Source)
	}
	if len(out) == 0 {
		t.Fatal("expected a message")
	}
}

func TestImportCycle(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.hsl")
	b := filepath.Join(dir, "b.hsl")
	os.WriteFile(a, []byte("---\nimports:\n  - b.hsl\n---\n(x = 1)\n"), 0o644)
	os.WriteFile(b, []byte("---\nimports:\n  - a.hsl\n---\n(y = 2)\n"), 0o644)
	if _, err := Transpile(a, DefaultRules()); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestMarkdownSpans(t *testing.T) {
	rules := DefaultRules()
	rules.Markdown = true
	src := "See `print(1)` and\n\n```python\nprint(2)\n```\n\n```#final\nprint(3)\n```\n\nThen `#note print(4)`.\n"
	got, err := Extract(src, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got != "print(1)\nprint(2)\nprint(3)\n#note print(4)\n" {
		t.Fatalf("got %q", got)
	}
	tags, err := FindTags(src, rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Name != "final" || tags[1].Name != "note" {
		t.Fatalf("tags %#v", tags)
	}
	if strings.Trim(tags[0].Text[tags[0].ContentStart-tags[0].Start:tags[0].ContentEnd-tags[0].Start], "\n") != "print(3)" {
		t.Fatal(tags[0].Text)
	}
	plain := DefaultRules()
	got, err = Extract("`print 1`", plain)
	if err != nil || got != "" {
		t.Fatalf("non-markdown extracted %q %v", got, err)
	}
}

func TestLispSpan(t *testing.T) {
	rules := rulesForInterpreter("sbcl", defaultDelims())
	got, err := Extract("((format t \"a; )\") ; )\n#| ) |#\n)\n", rules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `(format t "a; )")`) {
		t.Fatal(got)
	}
	got, err = Extract(`((write-char #\) ))`, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `#\)`) {
		t.Fatal(got)
	}
}

func TestSelect(t *testing.T) {
	src := "Title <#title hello>. Items <.item one> and (\n.item two\n). Again <.item three>."
	got, err := SelectText(src, Selector{Kind: '#', Name: "title"}, 0, DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if got != "<#title hello>\n" {
		t.Fatalf("id %q", got)
	}
	got, err = SelectText(src, Selector{Kind: '.', Name: "item"}, 0, DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if got != "<.item one>\n(\n.item two\n)\n<.item three>\n" {
		t.Fatalf("class %q", got)
	}
	got, err = SelectText(src, Selector{Kind: '.', Name: "item"}, 2, DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if got != "(\n.item two\n)\n" {
		t.Fatalf("nth %q", got)
	}
	next, err := ReplaceContent(src, Selector{Kind: '#', Name: "title"}, 0, "goodbye", DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next, "<#title goodbye>") {
		t.Fatal(next)
	}
	next, err = ReplaceContent(next, Selector{Kind: '.', Name: "item"}, 3, "four", DefaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next, "<.item four>") {
		t.Fatal(next)
	}
	if _, err := SelectText(src+" <#title extra>", Selector{Kind: '#', Name: "title"}, 0, DefaultRules()); err == nil {
		t.Fatal("duplicate id should fail")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
