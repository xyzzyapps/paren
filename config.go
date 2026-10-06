package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// fileConfig is hsl.toml, nearest file walking up from the document.
// An absent comment key keeps the interpreter preset.
type fileConfig struct {
	Delimiters    []string    `toml:"delimiters"`
	LineComments  *[]string   `toml:"line_comments"`
	BlockComments *[][]string `toml:"block_comments"`
}

func loadProjectConfig(start string) (fileConfig, string, error) {
	dir := start
	info, err := os.Stat(dir)
	if err != nil {
		return fileConfig{}, "", err
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		candidate := filepath.Join(dir, "hsl.toml")
		data, err := os.ReadFile(candidate)
		if err == nil {
			var cfg fileConfig
			if err := toml.Unmarshal(data, &cfg); err != nil {
				return fileConfig{}, candidate, fmt.Errorf("%s: %w", candidate, err)
			}
			return cfg, candidate, nil
		}
		if !os.IsNotExist(err) {
			return fileConfig{}, "", err
		}
		if isProjectRoot(dir) {
			return fileConfig{}, "", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fileConfig{}, "", nil
		}
		dir = parent
	}
}

func isProjectRoot(dir string) bool {
	for _, name := range []string{"go.mod", ".git"} {
		_, err := os.Stat(filepath.Join(dir, name))
		if err == nil {
			return true
		}
	}
	return false
}

func rulesFromConfig(cfg fileConfig, interpreter string, cliDelims []string) (Rules, error) {
	var openers map[byte]byte
	var err error
	switch {
	case len(cliDelims) > 0:
		openers, err = ParseDelims(cliDelims)
	case len(cfg.Delimiters) > 0:
		openers, err = ParseDelims(cfg.Delimiters)
	default:
		openers = defaultDelims()
	}
	if err != nil {
		return Rules{}, err
	}
	rules := rulesForInterpreter(interpreter, openers)
	if cfg.LineComments != nil {
		rules.Line = *cfg.LineComments
		rules.setLine = true
	}
	if cfg.BlockComments != nil {
		rules.Block = nil
		for _, pair := range *cfg.BlockComments {
			if len(pair) != 2 || pair[0] == "" || pair[1] == "" {
				return Rules{}, fmt.Errorf("block_comments entries are [open, close]")
			}
			rules.Block = append(rules.Block, [2]string{pair[0], pair[1]})
		}
	}
	return rules, nil
}
