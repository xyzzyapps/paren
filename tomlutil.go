package main

import "github.com/pelletier/go-toml/v2"

func tomlUnmarshal(data []byte, cfg *fileConfig) error {
	return toml.Unmarshal(data, cfg)
}
