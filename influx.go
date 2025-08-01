package pulse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NewInfluxStorage(influxUrl string) (*influxStorage, error) {

	baseUrl, err := url.Parse(influxUrl)
	if err != nil {
		return nil, err
	}

	switch baseUrl.Scheme {
	case "":
		baseUrl.Scheme = "http"
	case "http", "https":
		break
	default:
		return nil, fmt.Errorf("unsupported protocol scheme '%s'", baseUrl.Scheme)
	}

	this := influxStorage{baseUrl: url.URL{
		Scheme: baseUrl.Scheme,
		Host:   baseUrl.Host,
	}}

	//	this is stupid but the basic auth doesn't work here anyway;
	//	so for now we just grab any password that's provided and set it as the Token
	//	so yes, something like https://{bruh|token}:mytoken@example.com will totally work
	if pass, has := baseUrl.User.Password(); has {
		this.tokenAuth = &pass
	}

	if len(baseUrl.Path) < 2 {
		return nil, fmt.Errorf("database name missing in connection url")
	}

	if dbname, _, has := strings.Cut(baseUrl.Path[1:], "/"); has {
		return nil, fmt.Errorf("a connection url should not contain path elements")
	} else {
		this.dbName = dbname
	}

	if err := this.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}

	return &this, nil
}

type influxStorage struct {
	baseUrl   url.URL
	dbName    string
	tokenAuth *string
}

// Returns client TypeID
func (this *influxStorage) Type() string {
	return "influx"
}

// Returns client version
func (this *influxStorage) Version() string {
	return "v1"
}

func (this *influxStorage) fetch(ctx context.Context, method string, url *url.URL, body io.Reader) (*http.Response, error) {

	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}

	if this.tokenAuth != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Token %s", *this.tokenAuth))
	}

	return http.DefaultClient.Do(req.WithContext(ctx))
}

// Ping checks whether a database is reachable and ready to receive metrics writes.
// This is not a health check, the goal of Ping() is to ensure that the client is correctly initialized
func (this *influxStorage) Ping(ctx context.Context) error {

	params := this.baseUrl.Query()
	params.Add("q", "SHOW DATABASES")

	queryUrl := this.baseUrl
	queryUrl.Path = "/query"
	queryUrl.RawQuery = params.Encode()

	resp, err := this.fetch(ctx, "GET", &queryUrl, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {

		if body, err := io.ReadAll(resp.Body); err == nil {
			slog.Debug("INFLUX ping error",
				slog.Int("status", resp.StatusCode),
				slog.String("body", string(body)))
		}

		return influxFmtStatusError(resp.StatusCode)
	}

	return nil
}

// Writes a single uptime metric
func (this *influxStorage) WriteUptime(ctx context.Context, entry UptimeEntry) error {

	params := url.Values{}
	params.Set("db", this.dbName)

	pushUrl := this.baseUrl
	pushUrl.Path = "/write"
	pushUrl.RawQuery = params.Encode()

	liner := influxLiner{
		Labels: map[string]string{
			"probe":      entry.Label,
			"probe_type": entry.ProbeType,
		},
	}

	if entry.Host != nil {
		liner.Labels["host"] = *entry.Host
	}

	liner.WriteDuration("probe_elapsed", entry.ProbeElapsed)
	liner.WriteBool("up", entry.Up)
	liner.WriteDuration("latency", entry.FillLatency())
	liner.WriteInt("http_status", int64(entry.FillHttpStatus()))
	liner.WriteInt("tls_version", int64(entry.FillTlsVersion()))

	resp, err := this.fetch(ctx, "POST", &pushUrl, liner.Reader())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {

		if body, err := io.ReadAll(resp.Body); err == nil {
			slog.Debug("INFLUX: Request error",
				slog.String("body", string(body)))
		}

		return influxFmtStatusError(resp.StatusCode)
	}

	return nil
}

type influxLiner struct {
	Labels map[string]string

	builder strings.Builder
}

func (this *influxLiner) Reader() io.Reader {
	return strings.NewReader(this.builder.String())
}

func (this *influxLiner) write(key string, value any) {

	var line strings.Builder

	line.WriteString(url.QueryEscape(key))

	for key, val := range this.Labels {
		line.WriteString(fmt.Sprintf(",%s=%s", url.QueryEscape(key), url.QueryEscape(val)))
	}

	line.WriteString(fmt.Sprintf(" value=%d %d", value, time.Now().UnixNano()))

	if this.builder.Len() > 0 {
		this.builder.WriteRune('\n')
	}

	this.builder.WriteString(line.String())
}

func (this *influxLiner) WriteInt(key string, value int64) {
	this.write(key, value)
}

func (this *influxLiner) WriteDuration(key string, value time.Duration) {
	this.write(key, value.Milliseconds())
}

func (this *influxLiner) WriteFloat(key string, value float64) {
	this.write(key, value)
}

func (this *influxLiner) WriteBool(key string, value bool) {
	if value {
		this.write(key, 1)
	} else {
		this.write(key, 0)
	}
}

func influxFmtStatusError(status int) error {
	switch status {

	case http.StatusBadRequest:
		return errors.New("protocol error")

	case http.StatusUnauthorized:
		return errors.New("unauthorized")

	case http.StatusNotFound:
		return errors.New("not an influx endpoint")

	case http.StatusInternalServerError:
		return errors.New("internal server error")

	default:
		return fmt.Errorf("unexpected status code: %d", status)
	}
}
