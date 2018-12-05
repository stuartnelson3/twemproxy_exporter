package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "twemproxy_exporter"
)

func main() {
	var (
		timeout          = flag.Duration("twemproxy.timeout", 2*time.Second, "Timeout for request to twemproxy.")
		twemproxyAddress = flag.String("twemproxy.stats-address", "localhost:22222", "Stats address of twemproxy.")
		metricsPath      = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		listenAddress    = flag.String("web.listen-address", ":9151", "Address to listen on for web interface and telemetry.")
	)
	flag.Parse()

	log.Infof("listening on port %s path %s", *listenAddress, *metricsPath)

	u, err := url.Parse(*twemproxyAddress)
	if err != nil {
		log.Fatalf("failed to parse twemproxy.stats-address: %v", err)
	}
	if *timeout == 0 {
		*timeout = 2 * time.Second
	}

	prometheus.MustRegister(newExporter(u, *timeout))

	http.Handle(*metricsPath, prometheus.Handler())

	log.Infoln("starting twemproxy_exporter", version.Info())
	log.Infoln("build context", version.BuildContext())
	log.Infoln("listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

type exporter struct {
	endpoint *url.URL
	timeout  time.Duration

	up                 *prometheus.Desc
	totalConnections   *prometheus.Desc
	currentConnections *prometheus.Desc // How is this different from activeClientConnections?

	// Client metrics
	clientEOF               *prometheus.Desc
	clientErr               *prometheus.Desc
	activeClientConnections *prometheus.Desc // Double check this is a gauge, it sounds like one..
	// Number of times a backend server was ejected
	backendServerEjections *prometheus.Desc
	forwardErrors          *prometheus.Desc
	// Fragments created from a multi-vector request
	fragments *prometheus.Desc

	// Server metrics
	serverEOF               *prometheus.Desc
	serverErr               *prometheus.Desc
	serverTimeouts          *prometheus.Desc
	activeServerConnections *prometheus.Desc
	serverEjectedAt         *prometheus.Desc // Not mentioned in README, maybe unix timestamp of when server was ejected?
	requestCount            *prometheus.Desc
	requestBytes            *prometheus.Desc
	responseCount           *prometheus.Desc
	responseBytes           *prometheus.Desc
	incomingQueueGauge      *prometheus.Desc // Double check this is a gauge, it sounds like one..
	incomingQueueBytesGauge *prometheus.Desc // Double check this is a gauge, it sounds like one..
	outgoingQueueGauge      *prometheus.Desc // Double check this is a gauge, it sounds like one..
	outgoingQueueBytesGauge *prometheus.Desc // Double check this is a gauge, it sounds like one..
}

func newExporter(endpoint *url.URL, timeout time.Duration) *exporter {
	return &exporter{
		endpoint: endpoint,
		timeout:  timeout,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could twemproxy be queried.",
			nil,
			nil,
		),
		totalConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "connections_total"),
			"Total number of connections.",
			nil,
			nil,
		),
		currentConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "current_connections"),
			"The current number of connections.",
			nil,
			nil,
		),
		clientEOF: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "client_eof_total"),
			"Total number of client EOFs.",
			nil,
			nil,
		),
		clientErr: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "client_err_total"),
			"Total number of client errors.",
			nil,
			nil,
		),
		activeClientConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "active_client_connections"),
			"The current number of active client connections.",
			nil,
			nil,
		),
		backendServerEjections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "backend_server_ejections_total"),
			"The number of times a backend has been ejected.",
			nil,
			nil,
		),
		forwardErrors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "forward_errors_total"),
			"Total number of forward errors.",
			nil,
			nil,
		),
		fragments: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "fragments_total"),
			"Total number fragments created from multi-vector requests.",
			nil,
			nil,
		),
		serverEOF: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_eof_total"),
			"Total number of server EOFs.",
			[]string{"server"},
			nil,
		),
		serverErr: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_err_total"),
			"Total number of server errors.",
			[]string{"server"},
			nil,
		),
		serverTimeouts: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_timeouts_total"),
			"Total number of times the server has timed out.",
			[]string{"server"},
			nil,
		),
		activeServerConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_connections_active"),
			"The current number of active server connections.",
			[]string{"server"},
			nil,
		),
		serverEjectedAt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_ejected_at"),
			"The time when the server was ejected in UNIX time.", // Confirm ms/sec?
			[]string{"server"},
			nil,
		),
		requestCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_requests_total"),
			"Total number of requests to the server.",
			[]string{"server"},
			nil,
		),
		requestBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_requests_bytes_total"),
			"Total number of requests bytes sent to the server.",
			[]string{"server"},
			nil,
		),
		responseCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_responses_total"),
			"Total number of responses to the server.",
			[]string{"server"},
			nil,
		),
		responseBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_responses_bytes_total"),
			"Total number of response bytes sent to the server.",
			[]string{"server"},
			nil,
		),
		incomingQueueGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "incoming_queue"),
			"The current number of requests in the incoming queue.",
			[]string{"server"},
			nil,
		),
		incomingQueueBytesGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "incoming_queue_bytes"),
			"The current number of bytes in the incoming queue.",
			[]string{"server"},
			nil,
		),
		outgoingQueueGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "outgoing_queue"),
			"The current number of requests in the outgoing queue.",
			[]string{"server"},
			nil,
		),
		outgoingQueueBytesGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "outgoing_queue_bytes"),
			"The current number of bytes in the outgoing queue.",
			[]string{"server"},
			nil,
		),
	}
}

// Collect implements prometheus.Collector. It fetches the statistics from
// twemproxy, and delivers them as Prometheus metrics.
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	var up float64
	defer func() {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, up)
	}()
	st, err := e.collect()
	if err != nil {
		log.Infof("failed to collect stats: %v", err)
		return
	}

	up = 1

	ch <- prometheus.MustNewConstMetric(e.totalConnections, prometheus.CounterValue, st.TotalConnections)
	ch <- prometheus.MustNewConstMetric(e.currentConnections, prometheus.GaugeValue, st.CurrConnections)

	p := st.Proxied
	ch <- prometheus.MustNewConstMetric(e.clientEOF, prometheus.CounterValue, p.ClientEOF)
	ch <- prometheus.MustNewConstMetric(e.clientErr, prometheus.CounterValue, p.ClientErr)
	ch <- prometheus.MustNewConstMetric(e.activeClientConnections, prometheus.GaugeValue, p.ClientConnections)
	ch <- prometheus.MustNewConstMetric(e.backendServerEjections, prometheus.CounterValue, p.ClientConnections)
	ch <- prometheus.MustNewConstMetric(e.forwardErrors, prometheus.CounterValue, p.ForwardError)
	ch <- prometheus.MustNewConstMetric(e.fragments, prometheus.CounterValue, p.Fragments)

	for name, server := range p.Servers {
		ch <- prometheus.MustNewConstMetric(e.serverEOF, prometheus.CounterValue, server.ServerEOF, name)
		ch <- prometheus.MustNewConstMetric(e.serverErr, prometheus.CounterValue, server.ServerErr, name)
		ch <- prometheus.MustNewConstMetric(e.serverTimeouts, prometheus.CounterValue, server.ServerTimedout, name)
		ch <- prometheus.MustNewConstMetric(e.activeServerConnections, prometheus.GaugeValue, server.ServerConnections, name)
		ch <- prometheus.MustNewConstMetric(e.serverEjectedAt, prometheus.GaugeValue, server.ServerEjectedAt, name)
		ch <- prometheus.MustNewConstMetric(e.requestCount, prometheus.CounterValue, server.Requests, name)
		ch <- prometheus.MustNewConstMetric(e.requestBytes, prometheus.CounterValue, server.RequestBytes, name)
		ch <- prometheus.MustNewConstMetric(e.responseCount, prometheus.CounterValue, server.Responses, name)
		ch <- prometheus.MustNewConstMetric(e.responseBytes, prometheus.CounterValue, server.ResponseBytes, name)
		ch <- prometheus.MustNewConstMetric(e.incomingQueueGauge, prometheus.GaugeValue, server.InQueue, name)
		ch <- prometheus.MustNewConstMetric(e.incomingQueueBytesGauge, prometheus.GaugeValue, server.InQueueBytes, name)
		ch <- prometheus.MustNewConstMetric(e.outgoingQueueGauge, prometheus.GaugeValue, server.OutQueue, name)
		ch <- prometheus.MustNewConstMetric(e.outgoingQueueBytesGauge, prometheus.GaugeValue, server.OutQueueBytes, name)
	}
}

func (e *exporter) collect() (*stats, error) {
	// TODO: Reuse connection, connect with net.DialTCP?
	conn, err := net.DialTimeout("tcp", e.endpoint.String(), e.timeout)
	if err != nil {
		log.Errorf("failed to dial endpoint: %v", err)
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(e.timeout))
	defer conn.Close()

	r := bufio.NewReader(conn)

	st := new(stats)
	if err := json.NewDecoder(r).Decode(st); err != nil {
		log.Errorf("failed to unmarshal response: %v", err)
		return nil, err
	}

	return st, nil
}

// Describe implements prometheus.Collector. It sends all Descriptors possible
// from this exporter.
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.totalConnections
	ch <- e.currentConnections
	ch <- e.clientEOF
	ch <- e.clientErr
	ch <- e.activeClientConnections
	ch <- e.backendServerEjections
	ch <- e.forwardErrors
	ch <- e.fragments
	ch <- e.serverEOF
	ch <- e.serverErr
	ch <- e.serverTimeouts
	ch <- e.activeServerConnections
	ch <- e.serverEjectedAt
	ch <- e.requestCount
	ch <- e.requestBytes
	ch <- e.responseCount
	ch <- e.responseBytes
	ch <- e.incomingQueueGauge
	ch <- e.incomingQueueBytesGauge
	ch <- e.outgoingQueueGauge
	ch <- e.outgoingQueueBytesGauge
}

type stats struct {
	Service          string  `json:"service"`
	Source           string  `json:"source"`
	Version          string  `json:"version"`
	Uptime           float64 `json:"uptime"`
	Timestamp        float64 `json:"timestamp"`
	TotalConnections float64 `json:"total_connections"`
	CurrConnections  float64 `json:"curr_connections"`
	Proxied          Proxied `json:"proxied"`
}

type Proxied struct {
	ClientEOF         float64 `json:"client_eof"`
	ClientErr         float64 `json:"client_err"`
	ClientConnections float64 `json:"client_connections"`
	ServerEjects      float64 `json:"server_ejects"`
	ForwardError      float64 `json:"forward_error"`
	Fragments         float64 `json:"fragments"`
	Servers           map[string]*server
}

func (p *Proxied) UnmarshalJSON(b []byte) error {
	var dat map[string]interface{}
	if err := json.Unmarshal(b, &dat); err != nil {
		return err
	}
	p.ClientEOF = dat["client_eof"].(float64)
	delete(dat, "client_eof")
	p.ClientErr = dat["client_err"].(float64)
	delete(dat, "client_err")
	p.ClientConnections = dat["client_connections"].(float64)
	delete(dat, "client_connections")
	p.ServerEjects = dat["server_ejects"].(float64)
	delete(dat, "server_ejects")
	p.ForwardError = dat["forward_error"].(float64)
	delete(dat, "forward_error")
	p.Fragments = dat["fragments"].(float64)
	delete(dat, "fragments")
	p.Servers = make(map[string]*server, len(dat))

	for serverName, serverData := range dat {
		s := new(server)
		d, err := json.Marshal(serverData.(map[string]interface{}))
		if err != nil {
			return err
		}
		json.Unmarshal(d, s)
		p.Servers[serverName] = s
	}

	return nil
}

type server struct {
	ServerEOF         float64 `json:"server_eof"`
	ServerErr         float64 `json:"server_err"`
	ServerTimedout    float64 `json:"server_timedout"`
	ServerConnections float64 `json:"server_connections"`
	ServerEjectedAt   float64 `json:"server_ejected_at"`
	Requests          float64 `json:"requests"`
	RequestBytes      float64 `json:"request_bytes"`
	Responses         float64 `json:"responses"`
	ResponseBytes     float64 `json:"response_bytes"`
	InQueue           float64 `json:"in_queue"`
	InQueueBytes      float64 `json:"in_queue_bytes"`
	OutQueue          float64 `json:"out_queue"`
	OutQueueBytes     float64 `json:"out_queue_bytes"`
}
