package main

import "fmt"

// Rules says which bytes open a span and which comments hide closers.
type Rules struct {
	Openers map[byte]byte
	Closers map[byte]byte
	Line    []string
	Block   [][2]string
	Triple  bool
	Raw     bool
	Lisp     bool
	Markdown bool
	setLine  bool
}

func DefaultRules() Rules {
	return pythonRules(defaultDelims())
}

func defaultDelims() map[byte]byte {
	return map[byte]byte{'(': ')', '<': '>'}
}

func pythonRules(openers map[byte]byte) Rules {
	return Rules{
		Openers: openers,
		Closers: flip(openers),
		Line:    []string{"#"},
		Triple:  true,
		setLine: true,
	}
}

func nodeRules(openers map[byte]byte) Rules {
	return Rules{
		Openers: openers,
		Closers: flip(openers),
		Line:    []string{"//"},
		Block:   [][2]string{{"/*", "*/"}},
		Raw:     true,
		setLine: true,
	}
}

func slashRules(openers map[byte]byte) Rules {
	return Rules{
		Openers: openers,
		Closers: flip(openers),
		Line:    []string{"//"},
		Block:   [][2]string{{"/*", "*/"}},
		setLine: true,
	}
}

func goRules(openers map[byte]byte) Rules {
	r := slashRules(openers)
	r.Raw = true
	return r
}

func lispRules(openers map[byte]byte) Rules {
	return Rules{
		Openers: openers,
		Closers: flip(openers),
		Line:    []string{";"},
		Block:   [][2]string{{"#|", "|#"}},
		Lisp:    true,
		setLine: true,
	}
}

func rulesForInterpreter(interpreter string, openers map[byte]byte) Rules {
	switch interpreter {
	case "node", "nodejs", "deno", "bun":
		return nodeRules(openers)
	case "go":
		return goRules(openers)
	case "gcc", "clang", "cc", "tcc", "g++", "clang++":
		return slashRules(openers)
	case "sbcl", "clisp", "ecl":
		return lispRules(openers)
	default:
		return pythonRules(openers)
	}
}

func flip(openers map[byte]byte) map[byte]byte {
	out := make(map[byte]byte, len(openers))
	for open, close := range openers {
		out[close] = open
	}
	return out
}

// ParseDelims reads pairs such as "()" or a list of pairs.
func ParseDelims(parts []string) (map[byte]byte, error) {
	openers := map[byte]byte{}
	for _, part := range parts {
		if len(part)%2 != 0 || part == "" {
			return nil, fmt.Errorf("delimiter %q must be one or more pairs of bytes", part)
		}
		for i := 0; i < len(part); i += 2 {
			open, close := part[i], part[i+1]
			if open == close {
				return nil, fmt.Errorf("delimiter pair %q uses the same byte twice", part[i:i+2])
			}
			if _, ok := openers[open]; ok {
				return nil, fmt.Errorf("opener %q repeated", string(open))
			}
			for _, c := range openers {
				if c == close || c == open {
					return nil, fmt.Errorf("delimiter %q reuses a byte", part[i:i+2])
				}
			}
			openers[open] = close
		}
	}
	if len(openers) == 0 {
		return nil, fmt.Errorf("no delimiters")
	}
	return openers, nil
}

func (r Rules) delimBytes() string {
	b := make([]byte, 0, len(r.Openers)*2)
	for open, close := range r.Openers {
		b = append(b, open, close)
	}
	return string(b)
}
