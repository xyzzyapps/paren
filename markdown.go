package main

import (
	"fmt"
	"strings"
)

func isMarkdown(path string) bool {
	switch strings.ToLower(filepathExt(path)) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

func filepathExt(path string) string {
	i := strings.LastIndex(path, ".")
	slash := strings.LastIndexAny(path, `/\`)
	if i < 0 || i < slash {
		return ""
	}
	return path[i:]
}

// readMarkdown reads one fenced or inline code span starting at a backtick.
// A fence of three or more backticks uses the rest of that line as its info
// string. The info string is the delimiter, not the code. `#name` or `.name`
// there marks the span, the same way `(#name ...)` and `<.name ...>` do.
// A shorter run is inline code and ends at the next run of the same length.
func readMarkdown(source string, start int) (mdSpan, bool, error) {
	var none mdSpan
	if start >= len(source) || source[start] != '`' {
		return none, false, nil
	}
	if start > 0 && source[start-1] == '`' {
		return none, false, nil
	}
	n := 0
	for start+n < len(source) && source[start+n] == '`' {
		n++
	}
	if n >= 3 {
		return readFence(source, start, n)
	}
	return readInline(source, start, n)
}

type mdSpan struct {
	inner        string
	start        int
	end          int
	kind         byte
	name         string
	contentStart int
	contentEnd   int
	text         string
}

func readInline(source string, start, n int) (mdSpan, bool, error) {
	i := start + n
	for i < len(source) {
		if source[i] == '\\' && i+1 < len(source) && source[i+1] == '`' {
			i += 2
			continue
		}
		if source[i] == '`' {
			run := backtickRun(source, i)
			if run == n {
				inner := source[start+n : i]
				sp := mdSpan{
					inner:        inner,
					start:        start,
					end:          i + n,
					contentStart: start + n,
					contentEnd:   i,
					text:         source[start : i+n],
				}
				sp.kind, sp.name, sp.contentStart = tagPrefix(source, sp.contentStart, sp.contentEnd)
				return sp, true, nil
			}
			i += run
			continue
		}
		i++
	}
	line, col := location(source, start)
	return mdSpan{}, false, &ExtractError{Msg: fmt.Sprintf("unclosed %q at line %d, column %d", strings.Repeat("`", n), line, col)}
}

func readFence(source string, start, n int) (mdSpan, bool, error) {
	i := start + n
	infoStart := i
	nl := strings.IndexByte(source[i:], '\n')
	var bodyStart int
	var info string
	if nl < 0 {
		// Same-line fence: ```code```
		closer, ok := findRun(source, i, n)
		if !ok {
			line, col := location(source, start)
			return mdSpan{}, false, &ExtractError{Msg: fmt.Sprintf("unclosed fence at line %d, column %d", line, col)}
		}
		return fenceSpan(source, start, infoStart, infoStart, closer, n, ""), true, nil
	}
	info = strings.TrimSpace(source[infoStart : i+nl])
	bodyStart = i + nl + 1
	// A closing run on the opening line means there is no info string.
	if closer, ok := findRun(source[:bodyStart-1], infoStart, n); ok {
		return fenceSpan(source, start, infoStart, infoStart, closer, n, ""), true, nil
	}
	line := bodyStart
	for line <= len(source) {
		next := strings.IndexByte(source[line:], '\n')
		var row string
		var rowEnd int
		if next < 0 {
			row = source[line:]
			rowEnd = len(source)
		} else {
			row = source[line : line+next]
			rowEnd = line + next + 1
		}
		trim := strings.TrimSpace(row)
		run := 0
		for run < len(trim) && trim[run] == '`' {
			run++
		}
		if run >= n && run == len(trim) {
			return fenceSpan(source, start, infoStart, bodyStart, line, n, info), true, nil
		}
		if next < 0 {
			break
		}
		line = rowEnd
	}
	lineNo, col := location(source, start)
	return mdSpan{}, false, &ExtractError{Msg: fmt.Sprintf("unclosed fence at line %d, column %d", lineNo, col)}
}

func fenceSpan(source string, start, infoStart, bodyStart, closer, n int, info string) mdSpan {
	_ = infoStart
	_ = n
	inner := ""
	if bodyStart <= closer {
		inner = source[bodyStart:closer]
	}
	end := closer
	for end < len(source) && source[end] == '`' {
		end++
	}
	sp := mdSpan{
		inner:        inner,
		start:        start,
		end:          end,
		contentStart: bodyStart,
		contentEnd:   closer,
		text:         source[start:end],
	}
	if kind, name, ok := infoTag(info); ok {
		sp.kind = kind
		sp.name = name
	}
	return sp
}

func infoTag(info string) (byte, string, bool) {
	if info == "" || (info[0] != '#' && info[0] != '.') {
		return 0, "", false
	}
	name := info[1:]
	if !validName(name) {
		return 0, "", false
	}
	return info[0], name, true
}

func tagPrefix(source string, start, end int) (byte, string, int) {
	j := start
	for j < end && isGap(source[j]) {
		j++
	}
	if j >= end || (source[j] != '#' && source[j] != '.') {
		return 0, "", start
	}
	kind := source[j]
	k := j + 1
	for k < end && isNameByte(source[k]) {
		k++
	}
	if k == j+1 {
		return 0, "", start
	}
	content := k
	for content < end && isGap(source[content]) {
		content++
	}
	return kind, source[j+1 : k], content
}

func findRun(source string, from, n int) (int, bool) {
	for i := from; i < len(source); i++ {
		if source[i] != '`' {
			continue
		}
		if backtickRun(source, i) >= n && (n >= 3 || backtickRun(source, i) == n) {
			return i, true
		}
	}
	return 0, false
}

func backtickRun(source string, i int) int {
	n := 0
	for i+n < len(source) && source[i+n] == '`' {
		n++
	}
	return n
}
