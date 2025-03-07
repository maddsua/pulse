package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

type RootConfig struct {
	Probes    map[string]ProbeConfig `yaml:"probes" json:"probes"`
	Exporters ExportersConfig        `yaml:"exporters"  json:"exporters"`
	Proxies   ProxyConfigMap         `yaml:"proxies"  json:"proxies"`
	Taskhost  TaskhostConfig         `yaml:"taskhost"  json:"taskhost"`
}

type ProxyConfigMap map[string]*ProxyConfig

func (this *RootConfig) Validate() error {

	for key, val := range this.Proxies {

		if val == nil {
			delete(this.Proxies, key)
			continue
		}

		if err := val.Validate(); err != nil {
			return fmt.Errorf("invalid proxy '%s' config: %s", key, err.Error())
		}
	}

	for key, val := range this.Probes {
		if err := val.Validate(this.Proxies); err != nil {
			return fmt.Errorf("invalid probe '%s' config: %s", key, err.Error())
		}
	}

	return nil
}

type ProbeConfig struct {
	Http *HttpProbeConfig `yaml:"http" json:"http"`
	Tls  *TlsProbeConfig  `yaml:"tls" json:"tls"`
}

func (this *ProbeConfig) UptimeChecks() int {

	cases := []bool{
		this.Http != nil,
	}

	var count int

	for _, item := range cases {
		if item {
			count++
		}
	}

	return count
}

func (this *ProbeConfig) Validate(proxies ProxyConfigMap) error {

	var count int

	if this.Http != nil {

		count++

		if err := this.Http.Validate(); err != nil {
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

	if this.Tls != nil {

		count++

		if err := this.Tls.Validate(); err != nil {
			return fmt.Errorf("invalid tls probe config: %s", err.Error())
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

func (this *BaseProbeConfig) Validate() error {

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
	BaseProbeConfig
	Method  HttpMethod        `yaml:"method" json:"method"`
	Url     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Proxy   string            `yaml:"proxy" json:"proxy"`
}

func (this *HttpProbeConfig) Validate() error {

	if err := this.BaseProbeConfig.Validate(); err != nil {
		return fmt.Errorf("invalid probe base config '%s'", err.Error())
	}

	if !this.Method.Validate() {
		return fmt.Errorf("invalid http method '%s'", this.Method)
	}

	if _, err := url.Parse(this.Url); err != nil {
		return fmt.Errorf("invalid http url '%s'", this.Url)
	}

	return nil
}

type HttpMethod string

func (this *HttpMethod) Validate() bool {

	if *this == "" {
		*this = http.MethodHead
		return true
	}

	*this = HttpMethod(strings.ToUpper(string(*this)))
	return *this == http.MethodGet || *this == http.MethodHead || *this == http.MethodPost
}

type ExportersConfig struct {
	Web WebExporterConfig `yaml:"web" json:"web"`
}

func (this *ExportersConfig) HasHandlers() bool {

	cases := []bool{
		this.Web.Enabled,
	}

	var count int

	for _, item := range cases {
		if item {
			count++
		}
	}

	return count > 0
}

type WebExporterConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type ProxyConfig struct {
	Url string `yaml:"url" json:"url"`
}

func (this *ProxyConfig) Validate() error {

	if strings.HasPrefix(this.Url, "$") {

		url := os.Getenv(this.Url[1:])
		if url == "" {
			return fmt.Errorf("url variable '%s' is not defined", this.Url)
		}

		this.Url = url
	}

	parsedURL, err := url.Parse(this.Url)
	if err != nil {
		return fmt.Errorf("invalid proxy url: %s", err.Error())
	}

	switch strings.ToLower(parsedURL.Scheme) {
	case "socks", "socks4", "socks5":
	default:
		return fmt.Errorf("unsupported proxy protocol")
	}

	if parsedURL.Hostname() == "" {
		return fmt.Errorf("invalid proxy url: host name required")
	}

	if parsedURL.Port() == "" {
		return fmt.Errorf("invalid proxy url: port required")
	}

	return nil
}

type TaskhostConfig struct {
	Autorun bool `yaml:"autorun" json:"autorun"`
}

type TlsProbeConfig struct {
	BaseProbeConfig
	Host string `yaml:"host" json:"host"`
}

func (this *TlsProbeConfig) Validate() error {

	if err := this.BaseProbeConfig.Validate(); err != nil {
		return fmt.Errorf("invalid probe base config '%s'", err.Error())
	}

	if this.Host = strings.TrimSpace(this.Host); this.Host == "" {
		return errors.New("tls probe host is empty")
	}

	return nil
}
