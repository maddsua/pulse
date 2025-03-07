package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/maddsua/pulse/utils"
)

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
	CfgInterval string `yaml:"interval" json:"interval"`
	CfgTimeout  string `yaml:"timeout" json:"timeout"`
	interval    time.Duration
	timeout     time.Duration
}

func (this *BaseProbeConfig) Validate() error {

	if val, err := utils.ParseDuration(this.CfgInterval); err != nil {
		return err
	} else {
		this.interval = val
	}

	if val, err := utils.ParseDuration(this.CfgTimeout); err != nil {
		return err
	} else {
		this.timeout = val
	}

	if this.interval <= 0 {
		this.interval = 60 * time.Second
	}

	if this.timeout <= 0 {
		this.timeout = 10 * time.Second
	}

	return nil
}

func (this *BaseProbeConfig) Interval() time.Duration {
	return this.interval
}

func (this *BaseProbeConfig) Timeout() time.Duration {
	return this.timeout
}

type HttpProbeConfig struct {
	BaseProbeConfig `yaml:",inline"`
	Method          HttpMethod        `yaml:"method" json:"method"`
	Url             string            `yaml:"url" json:"url"`
	Headers         map[string]string `yaml:"headers" json:"headers"`
	Proxy           string            `yaml:"proxy" json:"proxy"`
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
	BaseProbeConfig `yaml:",inline"`
	Host            string `yaml:"host" json:"host"`
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
