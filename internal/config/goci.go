package config

import (
	"github.com/BurntSushi/toml"
)

// GociPipeline holds optional top-level metadata for the pipeline.
type GociPipeline struct {
	Name string `toml:"name"`
}

// GociStep describes a single CI step.
type GociStep struct {
	Name string   `toml:"name"`
	Cmd  []string `toml:"cmd"`
}

// GociConfig is the parsed representation of a .goci file.
//
// Supported format:
//
//	[pipeline]
//	name = "my-app"
//
//	[[steps]]
//	name = "install"
//	cmd  = ["npm", "install"]
//
//	[[steps]]
//	name = "test"
//	cmd  = ["npm", "test"]
type GociConfig struct {
	Pipeline GociPipeline `toml:"pipeline"`
	Steps    []GociStep   `toml:"steps"`
}

// LoadGociConfig reads .goci from the current working directory.
func LoadGociConfig() (*GociConfig, error) {
	return LoadGociConfigFrom(".goci")
}

// LoadGociConfigFrom reads a .goci file from the given path (useful for tests).
func LoadGociConfigFrom(path string) (*GociConfig, error) {
	var cfg GociConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
