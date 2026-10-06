package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

type cliOpts struct {
	delims []string
	config string
	silent bool
}

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	cmd := args[0]
	opts, rest, err := parseOpts(args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		usage()
		return 2
	}
	switch cmd {
	case "get":
		return cmdGet(rest, opts)
	case "set":
		return cmdSet(rest, opts)
	case "transpile", "run":
		if len(rest) < 1 {
			usage()
			return 2
		}
		return cmdScript(cmd, rest[0], rest[1:], opts)
	default:
		opts, rest, err = parseOpts(args)
		if err != nil || len(rest) < 1 {
			usage()
			return 2
		}
		return cmdScript("transpile", rest[0], rest[1:], opts)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: paren [transpile] [--silent] [--delims PAIRS] [--config hsl.toml] <file.hsl>")
	fmt.Fprintln(os.Stderr, "       paren run [--delims PAIRS] [--config hsl.toml] <file.hsl> [args...]")
	fmt.Fprintln(os.Stderr, "       paren get [--delims PAIRS] [--config hsl.toml] <file> <#id|.class> [--nth N]")
	fmt.Fprintln(os.Stderr, "       paren set [--delims PAIRS] [--config hsl.toml] <file> <#id|.class> [--nth N] <content>")
	fmt.Fprintln(os.Stderr, "PAIRS is one or more two-byte pairs, such as '()<>' or '[]'.")
	fmt.Fprintln(os.Stderr, "hsl.toml is read from the file's directory or a parent project directory.")
}

func parseOpts(args []string) (cliOpts, []string, error) {
	var opts cliOpts
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--delims":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--delims needs pairs")
			}
			i++
			opts.delims = append(opts.delims, args[i])
		case "--config":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--config needs a path")
			}
			i++
			opts.config = args[i]
		case "--silent":
			opts.silent = true
		default:
			rest = append(rest, args[i])
		}
	}
	return opts, rest, nil
}

func resolveRules(path, interpreter string, opts cliOpts) (Rules, error) {
	var cfg fileConfig
	var err error
	if opts.config != "" {
		data, readErr := os.ReadFile(opts.config)
		if readErr != nil {
			return Rules{}, readErr
		}
		if err := tomlUnmarshal(data, &cfg); err != nil {
			return Rules{}, fmt.Errorf("%s: %w", opts.config, err)
		}
	} else {
		cfg, _, err = loadProjectConfig(path)
		if err != nil {
			return Rules{}, err
		}
	}
	if interpreter == "" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return Rules{}, readErr
		}
		front, _, splitErr := splitFront(string(data))
		if splitErr != nil {
			return Rules{}, splitErr
		}
		interpreter = front.Interpreter
	}
	rules, err := rulesFromConfig(cfg, interpreter, opts.delims)
	if err != nil {
		return Rules{}, err
	}
	rules.Markdown = isMarkdown(path)
	return rules, nil
}

func cmdScript(cmd, path string, scriptArgs []string, opts cliOpts) int {
	rules, err := resolveRules(path, "", opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	script, err := Transpile(path, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	if cmd == "transpile" {
		if !opts.silent {
			os.Stdout.WriteString(script.Source)
		}
		return 0
	}
	return execute(script, path, scriptArgs)
}

func execute(script Script, sourcePath string, scriptArgs []string) int {
	switch script.Interpreter {
	case "go":
		return runTool(script, sourcePath, scriptArgs, func(bin, file string) []string {
			return append([]string{"run", file}, scriptArgs...)
		}, true)
	case "gcc", "clang", "cc", "tcc", "g++", "clang++":
		return runCompiled(script, sourcePath, scriptArgs)
	case "sbcl":
		return runTool(script, sourcePath, scriptArgs, func(bin, file string) []string {
			return append([]string{"--script", file}, scriptArgs...)
		}, false)
	case "ecl":
		return runTool(script, sourcePath, scriptArgs, func(bin, file string) []string {
			return append([]string{"--shell", file}, scriptArgs...)
		}, false)
	default:
		return runTool(script, sourcePath, scriptArgs, func(bin, file string) []string {
			return append([]string{file}, scriptArgs...)
		}, false)
	}
}

func runTool(script Script, sourcePath string, scriptArgs []string, argv func(bin, file string) []string, dirOfFile bool) int {
	bin, err := exec.LookPath(script.Interpreter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: interpreter %q is not on PATH\n", script.Interpreter)
		fmt.Fprintf(os.Stderr, "paren: transpiled script was not run; use `paren transpile` to print it\n")
		return 1
	}
	file, cleanup, err := writeTemp(script)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	defer cleanup()
	c := exec.Command(bin, argv(bin, file)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if dirOfFile {
		c.Dir = filepath.Dir(file)
	} else {
		c.Dir, _ = filepath.Abs(filepath.Dir(sourcePath))
	}
	return wait(c)
}

func runCompiled(script Script, sourcePath string, scriptArgs []string) int {
	bin, err := exec.LookPath(script.Interpreter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: interpreter %q is not on PATH\n", script.Interpreter)
		fmt.Fprintf(os.Stderr, "paren: transpiled script was not run; use `paren transpile` to print it\n")
		return 1
	}
	file, cleanup, err := writeTemp(script)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	defer cleanup()
	exe := file + ".out"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	defer os.Remove(exe)
	compile := exec.Command(bin, file, "-o", exe)
	compile.Stderr = os.Stderr
	compile.Stdout = os.Stdout
	if err := compile.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	c := exec.Command(exe, scriptArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir, _ = filepath.Abs(filepath.Dir(sourcePath))
	return wait(c)
}

func writeTemp(script Script) (string, func(), error) {
	tmp, err := os.CreateTemp("", "paren-*"+script.Suffix)
	if err != nil {
		return "", nil, err
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(script.Source); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", nil, err
	}
	return path, func() { os.Remove(path) }, nil
}

func wait(c *exec.Cmd) int {
	if err := c.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	return 0
}

func cmdGet(args []string, opts cliOpts) int {
	path, selRaw, nth, _, err := parseSelectArgs(args, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		usage()
		return 2
	}
	source, rules, err := readWithRules(path, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	sel, err := ParseSelector(selRaw, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	text, err := SelectText(string(source), sel, nth, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	os.Stdout.WriteString(text)
	return 0
}

func cmdSet(args []string, opts cliOpts) int {
	path, selRaw, nth, content, err := parseSelectArgs(args, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		usage()
		return 2
	}
	source, rules, err := readWithRules(path, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	sel, err := ParseSelector(selRaw, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	next, err := ReplaceContent(string(source), sel, nth, content, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	info, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, []byte(next), mode); err != nil {
		fmt.Fprintf(os.Stderr, "paren: %v\n", err)
		return 1
	}
	return 0
}

func readWithRules(path string, opts cliOpts) ([]byte, Rules, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, Rules{}, err
	}
	front, _, err := splitFront(string(source))
	if err != nil {
		return nil, Rules{}, err
	}
	rules, err := resolveRules(path, front.Interpreter, opts)
	if err != nil {
		return nil, Rules{}, err
	}
	return source, rules, nil
}

func parseSelectArgs(args []string, wantContent bool) (path, sel string, nth int, content string, err error) {
	var pos []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--nth" {
			if i+1 >= len(args) {
				return "", "", 0, "", fmt.Errorf("--nth needs a number")
			}
			i++
			n, convErr := fmt.Sscanf(args[i], "%d", &nth)
			if convErr != nil || n != 1 || nth < 1 {
				return "", "", 0, "", fmt.Errorf("--nth wants a positive integer")
			}
			continue
		}
		pos = append(pos, args[i])
	}
	need := 2
	if wantContent {
		need = 3
	}
	if len(pos) != need {
		return "", "", 0, "", fmt.Errorf("wrong arguments")
	}
	path, sel = pos[0], pos[1]
	if wantContent {
		content = pos[2]
	}
	return path, sel, nth, content, nil
}
