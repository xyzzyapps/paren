package main

import (
	"fmt"
	"strings"
)

// ExtractError is an unclosed code span or a stray closer in an HSL document.
type ExtractError struct {
	Msg string
}

func (e *ExtractError) Error() string { return e.Msg }

// Extract concatenates every configured span in source, in order.
// Each opener nests only with its own closer. Strings and comments inside a
// span hide delimiters. A backslash before a delimiter in prose is literal.
func Extract(source string, rules Rules) (string, error) {
	chunks, err := Chunks(source, rules)
	if err != nil || len(chunks) == 0 {
		return "", err
	}
	return strings.Join(chunks, "\n") + "\n", nil
}

// Chunks returns each span's inner text, in order.
func Chunks(source string, rules Rules) ([]string, error) {
	delims := rules.delimBytes()
	var chunks []string
	for i := 0; i < len(source); {
		ch := source[i]
		if ch == '\\' && i+1 < len(source) && (strings.ContainsRune(delims, rune(source[i+1])) || (rules.Markdown && source[i+1] == '`')) {
			i += 2
			continue
		}
		if rules.Markdown && ch == '`' {
			sp, ok, err := readMarkdown(source, i)
			if err != nil {
				return nil, err
			}
			if ok {
				chunks = append(chunks, strings.Trim(sp.inner, "\n"))
				i = sp.end
				continue
			}
		}
		if _, ok := rules.Openers[ch]; ok {
			inner, next, err := readSpan(source, i, rules)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, strings.Trim(inner, "\n"))
			i = next
			continue
		}
		if _, ok := rules.Closers[ch]; ok {
			line, col := location(source, i)
			return nil, &ExtractError{Msg: fmt.Sprintf("stray %q at line %d, column %d", ch, line, col)}
		}
		i++
	}
	if chunks == nil {
		chunks = []string{}
	}
	return chunks, nil
}

func readSpan(source string, start int, rules Rules) (string, int, error) {
	opener := source[start]
	closer := rules.Openers[opener]
	i := start + 1
	depth := 1
	var buf strings.Builder
	for i < len(source) {
		if lit, next, ok, err := takeLiteral(source, i, start, rules); ok || err != nil {
			if err != nil {
				return "", 0, err
			}
			buf.WriteString(lit)
			i = next
			continue
		}
		ch := source[i]
		if ch == opener {
			depth++
			buf.WriteByte(ch)
			i++
			continue
		}
		if ch == closer {
			depth--
			if depth == 0 {
				return buf.String(), i + 1, nil
			}
			buf.WriteByte(ch)
			i++
			continue
		}
		buf.WriteByte(ch)
		i++
	}
	line, col := location(source, start)
	return "", 0, &ExtractError{Msg: fmt.Sprintf("unclosed %q at line %d, column %d", opener, line, col)}
}

func takeLiteral(source string, i, spanStart int, rules Rules) (string, int, bool, error) {
	ch := source[i]
	if ch == '"' || ch == '\'' || (rules.Raw && ch == '`') {
		lit, next, err := readString(source, i, rules)
		return lit, next, true, err
	}
	if rules.Lisp {
		if lit, next, ok := lispToken(source, i); ok {
			return lit, next, true, nil
		}
	}
	if i == spanStart+1 && strings.HasPrefix(source[i:], "#") {
		return "", 0, false, nil
	}
	for _, block := range rules.Block {
		if strings.HasPrefix(source[i:], block[0]) {
			endRel := strings.Index(source[i+len(block[0]):], block[1])
			if endRel < 0 {
				line, col := location(source, i)
				return "", 0, false, &ExtractError{Msg: fmt.Sprintf("unclosed comment at line %d, column %d", line, col)}
			}
			end := i + len(block[0]) + endRel + len(block[1])
			return source[i:end], end, true, nil
		}
	}
	for _, line := range rules.Line {
		if line == "" || !strings.HasPrefix(source[i:], line) {
			continue
		}
		end := strings.IndexByte(source[i:], '\n')
		if end < 0 {
			return source[i:], len(source), true, nil
		}
		return source[i : i+end], i + end, true, nil
	}
	return "", 0, false, nil
}

// lispToken copies a character literal (#\A, #\Space) or a |barred| symbol.
func lispToken(source string, i int) (string, int, bool) {
	if strings.HasPrefix(source[i:], `#\`) && i+2 <= len(source) {
		j := i + 2
		if j < len(source) && isLispName(source[j]) {
			j++
			for j < len(source) && isLispName(source[j]) {
				j++
			}
		} else if j < len(source) {
			j++
		}
		return source[i:j], j, true
	}
	if source[i] != '|' {
		return "", 0, false
	}
	j := i + 1
	for j < len(source) {
		if source[j] == '\\' && j+1 < len(source) {
			j += 2
			continue
		}
		if source[j] == '|' {
			return source[i : j+1], j + 1, true
		}
		j++
	}
	return "", 0, false
}

func isLispName(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_'
}

func readString(source string, start int, rules Rules) (string, int, error) {
	quote := source[start]
	if quote == '`' {
		end := strings.IndexByte(source[start+1:], '`')
		if end < 0 {
			line, col := location(source, start)
			return "", 0, &ExtractError{Msg: fmt.Sprintf("unclosed string at line %d, column %d", line, col)}
		}
		return source[start : start+1+end+1], start + 1 + end + 1, nil
	}
	if rules.Triple && strings.HasPrefix(source[start:], strings.Repeat(string(quote), 3)) {
		marker := strings.Repeat(string(quote), 3)
		end := start + 3
		for end < len(source) {
			if strings.HasPrefix(source[end:], marker) {
				return source[start : end+3], end + 3, nil
			}
			if source[end] == '\\' {
				end += 2
				if end > len(source) {
					break
				}
				continue
			}
			end++
		}
		line, col := location(source, start)
		return "", 0, &ExtractError{Msg: fmt.Sprintf("unclosed string at line %d, column %d", line, col)}
	}
	end := start + 1
	for end < len(source) {
		ch := source[end]
		if ch == '\\' {
			end += 2
			if end > len(source) {
				break
			}
			continue
		}
		if ch == quote {
			return source[start : end+1], end + 1, nil
		}
		if ch == '\n' {
			break
		}
		end++
	}
	line, col := location(source, start)
	return "", 0, &ExtractError{Msg: fmt.Sprintf("unclosed string at line %d, column %d", line, col)}
}

func location(source string, index int) (int, int) {
	line := strings.Count(source[:index], "\n") + 1
	last := strings.LastIndexByte(source[:index], '\n')
	col := index + 1
	if last >= 0 {
		col = index - last
	}
	return line, col
}
