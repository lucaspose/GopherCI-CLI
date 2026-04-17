package config

import (
	"github.com/BurntSushi/toml"
)

type GociStep struct {
	Name string `toml:"name"`
	Cmd []string `toml:"cmd"`
}

type GociConfig struct {
	Steps []GociStep `toml:"steps"`
}
func LoadGociConfig() (*GociConfig, error) {
	var cfg GociConfig
	if _, err := toml.DecodeFile(".goci", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}