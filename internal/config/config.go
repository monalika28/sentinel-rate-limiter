package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	Descriptor    string `yaml:"descriptor"`
	Limit         int    `yaml:"limit"`
	WindowSeconds int    `yaml:"window_seconds"`
}

type Config struct {
	Rules []Rule `yaml:"rules"`
}

// LoadRules parses the rules.yaml and maps descriptors for fast lookup
func LoadRules(path string) (map[string]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	ruleMap := make(map[string]Rule)
	for _, r := range cfg.Rules {
		ruleMap[r.Descriptor] = r
	}
	return ruleMap, nil
}

func (r Rule) WindowDuration() time.Duration {
	return time.Duration(r.WindowSeconds) * time.Second
}