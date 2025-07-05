package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/maddsua/pulse"
	"gopkg.in/yaml.v3"
)

func FindConfig(locations []string) (string, bool) {

	for _, val := range locations {

		stat, err := os.Stat(val)
		if err != nil {
			continue
		}

		if stat.Mode().IsRegular() {
			return val, true
		}
	}

	return "", false
}

func LoadConfigFile(path string) (*FileConfig, error) {

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

	var cfg FileConfig

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

type Labeler interface {
	Labels() []string
}

type FileConfig struct {
	Probes  FileConfigProbesSecion `yaml:"probes" json:"probes"`
	Autorun bool                   `yaml:"autorun" json:"autorun"`
}

type FileConfigProbesSecion struct {
	Http ProbeConfig[pulse.HttpProbeOptions] `yaml:"http" json:"http"`
	Icmp ProbeConfig[pulse.IcmpProbeOptions] `yaml:"icmp" json:"icmp"`
}

type ProbeConfig[T any] map[string]T

func (this ProbeConfig[T]) Labels() []string {

	if this == nil {
		return nil
	}

	var labels []string
	for key := range this {
		labels = append(labels, key)
	}
	return labels
}
