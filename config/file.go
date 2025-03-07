package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadConfigFile(path string) (*RootConfig, error) {

	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %s", err.Error())
	}

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file info: %s", err.Error())
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("failed to read config file: config file must be a regular file")
	}

	var cfg RootConfig

	if strings.HasSuffix(path, ".yml") {
		if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %s", err.Error())
		}
	} else if strings.HasSuffix(path, ".json") {
		if err := json.NewDecoder(file).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %s", err.Error())
		}
	} else {
		return nil, errors.New("unsupported config file format")
	}

	return &cfg, nil
}
