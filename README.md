# paren

While writing in English allows prompting highly experimental and creative, it lacks the precission of programming. Can we capture the best of both worlds ?

English as a harness scripting language . You write prose. The program is the text inside delimiter pairs, concatenated in the order it appears. The default pairs are `()` and `<>`.

paren is a transpiler. `paren run` executes the result when the interpreter named in the document is on `PATH`. A C document is compiled with `gcc` (or `clang`, `cc`, `tcc`) and a C++ document with `g++` or `clang++`. A Go document is passed to `go run`. Common Lisp is run with `sbcl --script`, or with `clisp` or `ecl` when the frontmatter names those. 

## Potential Users

* Book Authors
* Scientists
* Maintenance Programmers

wherever literate programming helps.

## Install

The module is this directory.

```
go run . transpile examples/greet.hsl
go run . run examples/greet.hsl
```

Build a binary if you want `paren` on `PATH`:

```
go build -o paren .
```

The rest of this tutorial uses `go run .` so it works from a checkout.

## Simplest use case

1. Use parentheses in order. The program is the text inside each pair, concatenated from top to bottom. `paren notes.hsl` prints that script on standard output. `paren transpile notes.hsl` does the same.

```
Store a greeting with (greeting = "hello").
Show it with (print(greeting)).
```

2. Use tags when you need to search the source, or when an assistant should fill one block. `#id` is a single span. `.class` is a list of spans. `paren get` prints the match. `paren set` replaces the body and leaves the name in place.

```
<#sum total = 0>
<.step print("start")>
```

3. Use one `<:` ... `>` block when the program itself has to be mangled. That block is the body of the JavaScript function `mangle`. It can see the ids, the classes, the spans in order, the imports, and the YAML attributes, and the value it returns is the script.

```
<:
return parens.join("\n") + "\n";
>
```

`--silent` still transpiles the file, so `mangle` runs, and it writes nothing. Without `--silent`, paren prints the script.

```
go run . --silent notes.hsl
go run . transpile --silent notes.hsl
```

## A first document

Create `hello.hsl`:

```
---
interpreter: python
---
Store a greeting with (greeting = "hello").
Show it with (print(greeting)).
```

Transpile prints the script. Run executes it.

```
go run . transpile hello.hsl
go run . run hello.hsl
```

The script is:

```python
greeting = "hello"
print(greeting)
```

Parentheses nest with parentheses, and angle brackets nest with angle brackets. Inside one pair, the other pair is ordinary text, so an include or a comparison can use `<`:

```
(print("3 < 4 is", 3 < 4))
```

Quotes hide delimiters. So do comments: `#` to the end of the line for Python, and `//` plus `/* */` for Node, Go, and C. A backslash in prose writes a literal delimiter: `\(`, `\)`, `\<`, `\>`.

## Frontmatter and imports

A document may open with YAML between `---` lines.

```yaml
---
interpreter: python
suffix: .py
imports:
  - twice.hsl
---
```

`imports` are other paren files. Paths are relative to the file that lists them. Each import is expanded before the importer, depth first. A cycle is an error. `interpreter` is the program `paren run` looks up. `suffix` is the temporary file extension. When `suffix` is omitted, paren picks one from the interpreter (`.py`, `.js`, `.go`, `.c`, and a few others).

`examples/greet.hsl` imports `examples/twice.hsl` this way.

## Names

A span may carry an id or a class. Both delimiters work.

```
The title is <#title hello>.
Items are <.item one> and <.item two>.
The same mark works in parentheses: (#note remember this).
```

`paren get` prints the matching expression. `#id` is one value. `.class` is every match, one after another. `--nth` picks a single class match, counting from 1.

```
go run . get notes.hsl "#title"
go run . get notes.hsl ".item"
go run . get notes.hsl ".item" --nth 2
```

`paren set` replaces the body and leaves the name in place. It rewrites the file. A class needs `--nth`.

```
go run . set notes.hsl "#title" "goodbye"
go run . set notes.hsl ".item" --nth 1 "alpha"
```

## Filling blocks from an assistant

An id is one slot. A class is a family of slots. Leave the slot in the prose, with the name and a short note about what belongs there. An assistant reads the slot with `paren get`, writes code for that hole only, and puts it back with `paren set`. The name stays, so a later pass can find the same block.

Write the document with the holes marked and the surrounding program already in place:

```
---
interpreter: python
---
Load the two numbers the program adds.

<#left numbers = (3, 4)>

Add them. <#sum total = 0  # fill: total = numbers[0] + numbers[1]>

Show the total. (print(total))
```

Ask the assistant for one block. `#sum` is a single expression, so the reply is the body of that span and nothing else:

```
go run . get add.hsl "#sum"
```

The tool prints the whole span, name included:

```
<#sum total = 0  # fill: total = numbers[0] + numbers[1]>
```

The assistant replaces the body. The id is still `#sum` after the write:

```
go run . set add.hsl "#sum" "total = numbers[0] + numbers[1]"
```

The paragraph now holds the generated line, and the rest of the document is untouched. Transpile, then run:

```
go run . transpile add.hsl
go run . run add.hsl
```

Use a class when several holes share a shape. Each match is a separate place to fill. `--nth` counts from 1 in source order.

```
Each step is one line of the report.
<.step print("start")>
<.step print("middle")>
<.step print("end")>
```

List every step, then replace only the second:

```
go run . get report.hsl ".step"
go run . set report.hsl ".step" --nth 2 "print('between')"
```

`paren get` on `.step` prints each matching expression, one after another. `paren get` on `#sum` prints one expression. That text is what you paste into the prompt, along with the prose around it if the model needs the neighbors. Tell the model to answer with the body only, the code that sits inside the span, not a new wrapper and not the other spans.

A second pass reads the same name again. If the generated body is wrong, set that id or that nth class once more. The prose, the imports, and the other spans stay where they were.

## The mangle block

A file may contain one `<:` ... `>` block. The text inside it is the body of a JavaScript function named `mangle`. paren runs that function with Duktape while it transpiles the file. The block is not copied into the script. A second `<:` block in the same file is an error.

```javascript
function mangle(ids, classes, parens, imports, attrs) {
  // the <: > block is this body
}
```

`ids` maps an id to that one expression, so `ids.title` is `<#title hello>`. `classes` maps a class to the list of matching expressions, in source order. `parens` is every span body, in source order, and a multiline span is one entry. `imports` is the path list from the YAML frontmatter. `attrs` is the whole YAML mapping, including `interpreter` and any extra keys.

Return a string to replace this file's script. Return an array of strings to join them with newlines. Return nothing and paren joins `parens`, which is the ordinary extraction. Imports are still expanded before this file.

`<` and `>` inside the block nest. Quotes, `//` comments, and `/* */` comments hide them.

```
---
interpreter: python
note: keep
---
<#title hello>
(print(1))
<:
return ids.title + "\n" + attrs.note + "\n" + parens[1];
>
```

That file transpiles to:

```
<#title hello>
keep
print(1)
```

## As an Interface for LLMs Harnesses

By extracting an LLM prompt from an .hsl file using #ids, you can use English as the main interface for the harness. What ever is the output of the harness can be substituted back into the .hsl file for review. This will allow you to capture the entire prompt session of the project in a readable order.

An arbitrary Markdown document works the same way. In a `.md` or `.markdown` file, inline code in single or double backticks and a fence of three or more backticks are spans, in the same order as `()` and `<>`. The word on the opening fence is part of the marker, so a `python` fence contributes only the code inside it. Use `` `#name` `` or a fence info string of `#name` or `.name` when that span should be searchable. A backslash before a backtick leaves it as prose. Other file types do not treat backticks as spans. Parentheses still work in Markdown when you want them.

```
---
interpreter: python
---
# Notes

The loader comes first.

(
def load(path):
    return open(path).read()
)

## Report

Then the report.

(
print(load("in.txt"))
)
```

When the code is already inside fences and you do not want to rewrap every block, put the whole document in one span and let `mangle` pull the fences out. `ids.page` is that document. The function below keeps the text of each Python fence, in order, and drops the fence markers.

~~~~
---
interpreter: python
---
<#page
# Scratch

```python
print("from the notes")
```

More prose.

```python
print("and the second block")
```
>

<:
var text = ids.page;
var parts = [];
var fence = /```python\n([\s\S]*?)```/g;
var match;
while ((match = fence.exec(text))) parts.push(match[1].replace(/^\n|\n$/g, ""));
return parts.join("\n");
>
~~~~

`paren notes.md` prints the two `print` lines. `paren --silent notes.md` runs that same `mangle` and writes nothing. Change the fence label in the expression when the samples are not Python, and set `interpreter` to the program that should run them.

Pass pairs on the command line. Each pair is two bytes. Several pairs can sit in one argument, and the flag may be repeated. Command-line pairs replace the pairs from the config file. They do not add to them.

```
go run . transpile --delims "[]" examples/brackets/hello.hsl
go run . transpile --delims "()" --delims "<>" notes.hsl
```

Or put `hsl.toml` in the project. paren starts at the document's directory and walks toward the root. It uses the nearest `hsl.toml`. It stops at a directory that contains `go.mod` or `.git` when none of those directories had a config file. It never reads `~/hsl.toml`.

```toml
delimiters = ["[]"]
line_comments = ["#"]
block_comments = [["/*", "*/"]]
```

`examples/brackets/` is a small project that uses square brackets. Parentheses in that prose are not code.

`line_comments` and `block_comments` are optional. When they are absent, the interpreter chooses:

| interpreter | comments | strings |
| --- | --- | --- |
| `python`, `python3`, and the default | `#` | quotes, including triple quotes |
| `node`, `nodejs`, `deno`, `bun` | `//`, `/* */` | quotes and backticks |
| `go` | `//`, `/* */` | quotes and raw backticks |
| `gcc`, `clang`, `cc`, `tcc`, `g++`, `clang++` | `//`, `/* */` | quotes |
| `sbcl`, `clisp`, `ecl` | `;`, `#\| \|#` | quotes, `#\` characters, `\|barred\|` symbols |

`--config path/hsl.toml` loads that file instead of searching.

## Languages

Each file under `examples/langs/` is a complete program. A comment in the span contains a closer so the comment rule has to hold.

```
go run . run examples/langs/python.hsl
go run . run examples/langs/node.hsl
go run . run examples/langs/go.hsl
go run . run examples/langs/c.hsl
go run . run examples/langs/cpp.hsl
go run . run examples/langs/lisp.hsl
```

Python prints `hello from python`. Node prints `hello from node`. Go prints `hello from go`. C prints `hello from c`. C++ prints `hello from c++`. Common Lisp prints `hello from lisp`. `paren run` skips nothing on its own: if the named program is missing, it tells you and leaves the script unrun. `paren transpile` still prints it.

The C document names `gcc`. Use `clang`, `cc`, or `tcc` in the frontmatter if that is the compiler on `PATH`. The C++ document names `g++`; use `clang++` when that is what you have. The Lisp document names `sbcl`. The Go document must be one `package main` program, because `paren run` calls `go run` on the transpiled file.

## Tests

```
go test ./...
```

Language tests skip an interpreter that is not installed.
