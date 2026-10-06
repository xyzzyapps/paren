package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/olebedev/go-duktape.v3"
)

// stripMangle removes the single <: ... > block. The returned script is nil
// when the file has no mangle block. The block body is the source of function
// mangle, not part of the transpiled program.
func stripMangle(source string, rules Rules) (string, *string, error) {
	var b strings.Builder
	var body *string
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<:") {
			if body != nil {
				line, col := location(source, i)
				return "", nil, &ExtractError{Msg: fmt.Sprintf("second <: block at line %d, column %d", line, col)}
			}
			inner, next, err := readMangle(source, i)
			if err != nil {
				return "", nil, err
			}
			body = &inner
			i = next
			continue
		}
		if rules.Markdown && source[i] == '`' {
			sp, ok, err := readMarkdown(source, i)
			if err != nil {
				return "", nil, err
			}
			if ok {
				b.WriteString(source[i:sp.end])
				i = sp.end
				continue
			}
		}
		if _, ok := rules.Openers[source[i]]; ok {
			_, next, err := readSpan(source, i, rules)
			if err != nil {
				return "", nil, err
			}
			b.WriteString(source[i:next])
			i = next
			continue
		}
		b.WriteByte(source[i])
		i++
	}
	return b.String(), body, nil
}

func readMangle(source string, start int) (string, int, error) {
	i := start + 2
	depth := 1
	for i < len(source) {
		if lit, next, ok, err := jsLiteral(source, i); ok || err != nil {
			if err != nil {
				return "", 0, err
			}
			_ = lit
			i = next
			continue
		}
		switch source[i] {
		case '<':
			depth++
			i++
		case '>':
			depth--
			if depth == 0 {
				return source[start+2 : i], i + 1, nil
			}
			i++
		default:
			i++
		}
	}
	line, col := location(source, start)
	return "", 0, &ExtractError{Msg: fmt.Sprintf("unclosed <: at line %d, column %d", line, col)}
}

func jsLiteral(source string, i int) (string, int, bool, error) {
	if strings.HasPrefix(source[i:], "/*") {
		end := strings.Index(source[i+2:], "*/")
		if end < 0 {
			line, col := location(source, i)
			return "", 0, false, &ExtractError{Msg: fmt.Sprintf("unclosed comment at line %d, column %d", line, col)}
		}
		n := i + 2 + end + 2
		return source[i:n], n, true, nil
	}
	if strings.HasPrefix(source[i:], "//") {
		end := strings.IndexByte(source[i:], '\n')
		if end < 0 {
			return source[i:], len(source), true, nil
		}
		return source[i : i+end], i + end, true, nil
	}
	ch := source[i]
	if ch == '"' || ch == '\'' || ch == '`' {
		lit, next, err := readString(source, i, Rules{Raw: true})
		return lit, next, true, err
	}
	return "", 0, false, nil
}

func runMangle(jsBody, stripped string, front Front, rules Rules) (string, error) {
	ids := map[string]string{}
	classes := map[string][]string{}
	tags, err := FindTags(stripped, rules)
	if err != nil {
		return "", err
	}
	for _, tag := range tags {
		if tag.Kind == '#' {
			if _, ok := ids[tag.Name]; ok {
				return "", fmt.Errorf("#%s matches more than one expression", tag.Name)
			}
			ids[tag.Name] = tag.Text
			continue
		}
		classes[tag.Name] = append(classes[tag.Name], tag.Text)
	}
	parens, err := Chunks(stripped, rules)
	if err != nil {
		return "", err
	}
	attrs := front.Attrs
	if attrs == nil {
		attrs = map[string]any{}
	}
	imps := front.Imports
	if imps == nil {
		imps = []string{}
	}
	payload := map[string]any{
		"ids":     ids,
		"classes": classes,
		"parens":  parens,
		"imports": imps,
		"attrs":   attrs,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	ctx := duktape.New()
	defer ctx.DestroyHeap()
	program := "function mangle(ids, classes, parens, imports, attrs) {\n" + jsBody + "\n}\n" +
		"var __doc = " + string(raw) + ";\n" +
		"var __out = mangle(__doc.ids, __doc.classes, __doc.parens, __doc.imports, __doc.attrs);\n" +
		"if (typeof __out === 'undefined' || __out === null) {\n" +
		"  __out = __doc.parens.join('\\n');\n" +
		"  if (__doc.parens.length) __out += '\\n';\n" +
		"} else if (Object.prototype.toString.call(__out) === '[object Array]') {\n" +
		"  __out = __out.join('\\n');\n" +
		"  if (__out.length && __out.charAt(__out.length - 1) !== '\\n') __out += '\\n';\n" +
		"} else if (typeof __out !== 'string') {\n" +
		"  throw new Error('mangle must return a string, an array of strings, or nothing');\n" +
		"} else if (__out.length && __out.charAt(__out.length - 1) !== '\\n') {\n" +
		"  __out += '\\n';\n" +
		"}\n" +
		"__out;\n"
	if err := ctx.PevalString(program); err != nil {
		return "", fmt.Errorf("mangle: %s", ctx.SafeToString(-1))
	}
	return ctx.SafeToString(-1), nil
}
