package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type RootConfig struct {
	Probes    map[string]ProbeConfig `yaml:"probes"`
	Exporters ExportersConfig        `yaml:"exporters"`
}

func (this *RootConfig) Valid() error {

	for key, val := range this.Probes {
		if err := val.Valid(); err != nil {
			return fmt.Errorf("invalid probe '%s' config: %s", key, err.Error())
		}
	}

	return nil
}

type ProbeConfig struct {
	Http *HttpProbeConfig `yaml:"http"`
}

func (this *ProbeConfig) Stacks() int {

	states := []bool{
		this.Http != nil,
	}

	var count int

	for _, item := range states {
		if item {
			count++
		}
	}

	return count
}

func (this *ProbeConfig) Valid() error {

	var count int

	if this.Http != nil {

		count++

		if err := this.Http.Valid(); err != nil {
			return fmt.Errorf("invalid http probe config: %s", err.Error())
		}
	}

	if count == 0 {
		return errors.New("no probe target configs")
	}

	return nil
}

type BaseProbeConfig struct {
	Interval int `yaml:"interval"`
	Timeout  int `yaml:"timeout"`
}

func (this *BaseProbeConfig) Valid() error {

	if this.Interval == 0 {
		this.Interval = 60
	} else if this.Interval < 0 {
		return errors.New("invalid interval value")
	}

	if this.Timeout == 0 {
		this.Timeout = 10
	} else if this.Timeout < 0 {
		return errors.New("invalid timeout value")
	}

	return nil
}

type HttpProbeConfig struct {
	Method  HttpMethod        `yaml:"method"`
	Url     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	BaseProbeConfig
}

func (this *HttpProbeConfig) Valid() error {

	if !this.Method.Valid() {
		return fmt.Errorf("invalid http method '%s'", this.Method)
	}

	if _, err := url.Parse(this.Url); err != nil {
		return fmt.Errorf("invalid http url '%s'", this.Url)
	}

	if err := this.BaseProbeConfig.Valid(); err != nil {
		return fmt.Errorf("invalid prove base config '%s'", err.Error())
	}

	return nil
}

type HttpMethod string

func (this *HttpMethod) Valid() bool {

	if *this == "" {
		*this = http.MethodHead
		return true
	}

	*this = HttpMethod(strings.ToUpper(string(*this)))
	return *this == http.MethodGet || *this == http.MethodHead || *this == http.MethodPost
}

type ExportersConfig struct {
	Series bool `yaml:"series"`
}
