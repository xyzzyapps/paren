package main

import (
	"fmt"
	"strings"
	"unicode"
)

// Tag is one <#id ...> or <.class ...> span. Parentheses work the same way.
// Text is the whole delimited expression. Content is the text after the name.
type Tag struct {
	Kind         byte
	Name         string
	Start        int
	End          int
	ContentStart int
	ContentEnd   int
	Text         string
}

// FindTags lists every id or class span in source, outer and nested.
func FindTags(source string, rules Rules) ([]Tag, error) {
	var tags []Tag
	for i := 0; i < len(source); i++ {
		if rules.Markdown && source[i] == '`' {
			sp, ok, err := readMarkdown(source, i)
			if err != nil {
				return nil, err
			}
			if ok {
				if sp.kind != 0 {
					tags = append(tags, Tag{
						Kind:         sp.kind,
						Name:         sp.name,
						Start:        sp.start,
						End:          sp.end,
						ContentStart: sp.contentStart,
						ContentEnd:   sp.contentEnd,
						Text:         sp.text,
					})
				}
				if sp.end > i {
					i = sp.end - 1
				}
			}
			continue
		}
		if _, ok := rules.Openers[source[i]]; !ok {
			continue
		}
		tag, ok, err := tagAt(source, i, rules)
		if err != nil {
			return nil, err
		}
		if ok {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func tagAt(source string, start int, rules Rules) (Tag, bool, error) {
	j := start + 1
	for j < len(source) && isGap(source[j]) {
		j++
	}
	if j >= len(source) || (source[j] != '#' && source[j] != '.') {
		return Tag{}, false, nil
	}
	kind := source[j]
	nameStart := j + 1
	nameEnd := nameStart
	for nameEnd < len(source) && isNameByte(source[nameEnd]) {
		nameEnd++
	}
	if nameEnd == nameStart {
		return Tag{}, false, nil
	}
	_, end, err := readSpan(source, start, rules)
	if err != nil {
		return Tag{}, false, err
	}
	contentStart := nameEnd
	for contentStart < end-1 && isGap(source[contentStart]) {
		contentStart++
	}
	return Tag{
		Kind:         kind,
		Name:         source[nameStart:nameEnd],
		Start:        start,
		End:          end,
		ContentStart: contentStart,
		ContentEnd:   end - 1,
		Text:         source[start:end],
	}, true, nil
}

func isNameByte(b byte) bool {
	return b == '-' || b == '_' || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

// Selector is #id (one span) or .class (every span with that class).
type Selector struct {
	Kind byte
	Name string
}

func ParseSelector(raw string, rules Rules) (Selector, error) {
	s := strings.TrimSpace(raw)
	if len(s) >= 2 {
		if close, ok := rules.Openers[s[0]]; ok && s[len(s)-1] == close {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	if s == "" || (s[0] != '#' && s[0] != '.') {
		return Selector{}, fmt.Errorf("selector must be #id or .class")
	}
	name := s[1:]
	if !validName(name) {
		return Selector{}, fmt.Errorf("invalid selector %q", raw)
	}
	return Selector{Kind: s[0], Name: name}, nil
}

func matching(tags []Tag, sel Selector) []Tag {
	var out []Tag
	for _, t := range tags {
		if t.Kind == sel.Kind && t.Name == sel.Name {
			out = append(out, t)
		}
	}
	return out
}

// SelectText returns the one #id expression, or each .class expression.
func SelectText(source string, sel Selector, nth int, rules Rules) (string, error) {
	tags, err := FindTags(source, rules)
	if err != nil {
		return "", err
	}
	hits := matching(tags, sel)
	if sel.Kind == '#' {
		if nth != 0 {
			return "", fmt.Errorf("#%s is a single expression; omit --nth", sel.Name)
		}
		if len(hits) == 0 {
			return "", fmt.Errorf("#%s not found", sel.Name)
		}
		if len(hits) > 1 {
			return "", fmt.Errorf("#%s matches %d expressions", sel.Name, len(hits))
		}
		return withNL(hits[0].Text), nil
	}
	if nth != 0 {
		if nth < 1 || nth > len(hits) {
			return "", fmt.Errorf(".%s has %d matches; --nth %d is out of range", sel.Name, len(hits), nth)
		}
		return withNL(hits[nth-1].Text), nil
	}
	var b strings.Builder
	for _, h := range hits {
		b.WriteString(withNL(h.Text))
	}
	return b.String(), nil
}

func withNL(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// ReplaceContent replaces the body of #id, or the 1-based nth .class.
func ReplaceContent(source string, sel Selector, nth int, content string, rules Rules) (string, error) {
	tags, err := FindTags(source, rules)
	if err != nil {
		return "", err
	}
	hits := matching(tags, sel)
	var hit Tag
	switch sel.Kind {
	case '#':
		if nth != 0 {
			return "", fmt.Errorf("#%s is a single expression; omit --nth", sel.Name)
		}
		if len(hits) != 1 {
			return "", fmt.Errorf("#%s matches %d expressions", sel.Name, len(hits))
		}
		hit = hits[0]
	default:
		if nth == 0 {
			return "", fmt.Errorf(".%s needs --nth", sel.Name)
		}
		if nth < 1 || nth > len(hits) {
			return "", fmt.Errorf(".%s has %d matches; --nth %d is out of range", sel.Name, len(hits), nth)
		}
		hit = hits[nth-1]
	}
	body := content
	if hit.ContentStart == nameEnd(source, hit) && content != "" {
		body = " " + content
	}
	return source[:hit.ContentStart] + body + source[hit.ContentEnd:], nil
}

func nameEnd(source string, hit Tag) int {
	i := hit.Start + 1
	for i < hit.End && isGap(source[i]) {
		i++
	}
	if i < hit.End && (source[i] == '#' || source[i] == '.') {
		i++
	}
	for i < hit.End && isNameByte(source[i]) {
		i++
	}
	return i
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i]) {
			return false
		}
	}
	return true
}

func isGap(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
