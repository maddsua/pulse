package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type RootConfig struct {
	Probes    map[string]ProbeConfig `yaml:"probes" json:"probes"`
	Exporters ExportersConfig        `yaml:"exporters"  json:"exporters"`
	Proxies   ProxyConfigMap         `yaml:"proxies"  json:"proxies"`
}

type ProxyConfigMap map[string]ProxyConfig

func (this *RootConfig) Valid() error {

	for key, val := range this.Proxies {
		if err := val.Valid(); err != nil {
			return fmt.Errorf("invalid proxy '%s' config: %s", key, err.Error())
		}
	}

	for key, val := range this.Probes {
		if err := val.Valid(this.Proxies); err != nil {
			return fmt.Errorf("invalid probe '%s' config: %s", key, err.Error())
		}
	}

	return nil
}

type ProbeConfig struct {
	Http *HttpProbeConfig `yaml:"http" json:"http"`
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

func (this *ProbeConfig) Valid(proxies ProxyConfigMap) error {

	var count int

	if this.Http != nil {

		count++

		if err := this.Http.Valid(); err != nil {
			return fmt.Errorf("invalid http probe config: %s", err.Error())
		}

		if this.Http.Proxy != "" {

			if len(proxies) == 0 {
				return errors.New("no proxies defined in the config")
			}

			if _, has := proxies[this.Http.Proxy]; !has {
				return fmt.Errorf("probe proxy '%s' is not defined", this.Http.Proxy)
			}
		}
	}

	if count == 0 {
		return errors.New("no probe target configs")
	}

	return nil
}

type BaseProbeConfig struct {
	Interval int `yaml:"interval" json:"interval"`
	Timeout  int `yaml:"timeout" json:"timeout"`
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
	Method  HttpMethod        `yaml:"method" json:"method"`
	Url     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Proxy   string            `yaml:"proxy" json:"proxy"`
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
	Series bool `yaml:"series" json:"series"`
}

type ProxyConfig struct {
	Url string `yaml:"url" json:"url"`
}

func (this *ProxyConfig) Valid() error {

	if strings.HasPrefix(this.Url, "$") {

		if url := os.Getenv(this.Url[1:]); url == "" {
			return fmt.Errorf("url variable '%s' is not defined", this.Url)
		} else {
			this.Url = url
		}
	}

	if _, err := url.Parse(this.Url); err != nil {
		return fmt.Errorf("invalid proxy url: %s", err.Error())
	}

	return nil
}
