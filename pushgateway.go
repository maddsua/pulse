package pulse

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewPushgatewayStorage(hostUrl string) (*pushgatewayStorage, error) {

	baseUrl, err := url.Parse(hostUrl)
	if err != nil {
		return nil, err
	}

	if baseUrl.Host == "" {
		return nil, fmt.Errorf("missing url host")
	}

	switch baseUrl.Scheme {
	case "":
		baseUrl.Scheme = "http"
	case "http", "https":
		break
	default:
		return nil, fmt.Errorf("unsupported protocol scheme '%s'", baseUrl.Scheme)
	}

	this := &pushgatewayStorage{hostUrl: url.URL{
		Scheme: baseUrl.Scheme,
		Host:   baseUrl.Host,
		User:   baseUrl.User,
	}}

	if err := this.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}

	return this, nil
}

type pushgatewayStorage struct {
	hostUrl url.URL
}

func (this *pushgatewayStorage) Type() string {
	return "prometheus"
}

func (this *pushgatewayStorage) Version() string {
	return "v1"
}

func (this *pushgatewayStorage) Ping(ctx context.Context) error {

	pingUrl := this.hostUrl
	pingUrl.Path = "/api/v1/status"

	req, err := http.NewRequest("GET", pingUrl.String(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (this *pushgatewayStorage) WriteUptime(ctx context.Context, entry UptimeEntry) error {

	pushUrl := this.hostUrl
	pushUrl.Path = "/metrics/job/pulse"

	var addLabel = func(key, val string) {
		pushUrl.Path += fmt.Sprintf("/%s/%s", url.PathEscape(key), url.PathEscape(val))
	}

	addLabel("probe", entry.Label)
	addLabel("probe_type", entry.ProbeType)

	var liner pushgatewayLiner

	liner.WriteDuration("probe_elapsed", entry.ProbeElapsed)
	liner.WriteBool("up", entry.Up)

	if entry.HttpStatus != nil {
		liner.WriteInt("http_status", int64(*entry.HttpStatus))
	}

	if entry.Latency != nil {
		liner.WriteDuration("latency", *entry.Latency)
	}

	if entry.TlsVersion != nil {
		liner.WriteInt("tls_version", int64(*entry.TlsVersion))
	}

	if entry.Host != nil {
		addLabel("host", *entry.Host)
	}

	req, err := http.NewRequest("POST", pushUrl.String(), liner.Reader())
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode > 300 {

		if body, err := io.ReadAll(resp.Body); err == nil {
			slog.Debug("PUSHGATEWAY: Request error",
				slog.String("body", string(body)))
		}

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

type pushgatewayLiner struct {
	builder strings.Builder
}

func (this *pushgatewayLiner) Reader() io.Reader {
	return strings.NewReader(this.builder.String())
}

func (this *pushgatewayLiner) addLine(key, val string) {
	this.builder.WriteString(fmt.Sprintf("%s %s\n", key, val))
}

func (this *pushgatewayLiner) WriteInt(key string, val int64) {
	this.addLine(key, strconv.FormatInt(val, 10))
}

func (this *pushgatewayLiner) WriteDuration(key string, val time.Duration) {
	this.WriteInt(key, val.Milliseconds())
}

func (this *pushgatewayLiner) WriteBool(key string, val bool) {
	if val {
		this.WriteInt(key, 1)
	} else {
		this.WriteInt(key, 0)
	}
}
