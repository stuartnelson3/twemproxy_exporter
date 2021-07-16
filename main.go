package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"net"
	"net/http"
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

	if *twemproxyAddress == "" {
		log.Fatalln("-twemproxy.stats-address must not be blank")
	}

	if *timeout == 0 {
		*timeout = 2 * time.Second
	}

	prometheus.MustRegister(newExporter(*twemproxyAddress, *timeout))

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>Twemproxy Exporter</title></head>
		<body>
		<h1>Twemproxy Exporter</h1>
		<p><a href="` + *metricsPath + `">Metrics</a></p>
		</body>
		</html>`))
	})

	log.Infoln("starting twemproxy_exporter", version.Info())
	log.Infoln("build context", version.BuildContext())
	log.Infoln("listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

type exporter struct {
	endpoint string
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

func newExporter(endpoint string, timeout time.Duration) *exporter {
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
			[]string{"pool"},
			nil,
		),
		clientErr: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "client_err_total"),
			"Total number of client errors.",
			[]string{"pool"},
			nil,
		),
		activeClientConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "client_connections_active"),
			"The current number of active client connections.",
			[]string{"pool"},
			nil,
		),
		backendServerEjections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "backend_server_ejections_total"),
			"The number of times a backend has been ejected.",
			[]string{"pool"},
			nil,
		),
		forwardErrors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "forward_errors_total"),
			"Total number of forward errors.",
			[]string{"pool"},
			nil,
		),
		fragments: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "fragments_total"),
			"Total number fragments created from multi-vector requests.",
			[]string{"pool"},
			nil,
		),
		serverEOF: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_eof_total"),
			"Total number of server EOFs.",
			[]string{"pool", "server"},
			nil,
		),
		serverErr: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_err_total"),
			"Total number of server errors.",
			[]string{"pool", "server"},
			nil,
		),
		serverTimeouts: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_timeouts_total"),
			"Total number of times the server has timed out.",
			[]string{"pool", "server"},
			nil,
		),
		activeServerConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_connections_active"),
			"The current number of active server connections.",
			[]string{"pool", "server"},
			nil,
		),
		serverEjectedAt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_ejected_at"),
			"The time when the server was ejected in UNIX time.", // Confirm ms/sec?
			[]string{"pool", "server"},
			nil,
		),
		requestCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_requests_total"),
			"Total number of requests to the server.",
			[]string{"pool", "server"},
			nil,
		),
		requestBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_requests_bytes_total"),
			"Total number of requests bytes sent to the server.",
			[]string{"pool", "server"},
			nil,
		),
		responseCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_responses_total"),
			"Total number of responses to the server.",
			[]string{"pool", "server"},
			nil,
		),
		responseBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "server_responses_bytes_total"),
			"Total number of response bytes sent to the server.",
			[]string{"pool", "server"},
			nil,
		),
		incomingQueueGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "incoming_queue"),
			"The current number of requests in the incoming queue.",
			[]string{"pool", "server"},
			nil,
		),
		incomingQueueBytesGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "incoming_queue_bytes"),
			"The current number of bytes in the incoming queue.",
			[]string{"pool", "server"},
			nil,
		),
		outgoingQueueGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "outgoing_queue"),
			"The current number of requests in the outgoing queue.",
			[]string{"pool", "server"},
			nil,
		),
		outgoingQueueBytesGauge: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "outgoing_queue_bytes"),
			"The current number of bytes in the outgoing queue.",
			[]string{"pool", "server"},
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

	for poolName, pool := range st.Pools {
		ch <- prometheus.MustNewConstMetric(e.clientEOF, prometheus.CounterValue, pool.ClientEOF, poolName)
		ch <- prometheus.MustNewConstMetric(e.clientErr, prometheus.CounterValue, pool.ClientErr, poolName)
		ch <- prometheus.MustNewConstMetric(e.activeClientConnections, prometheus.GaugeValue, pool.ClientConnections, poolName)
		ch <- prometheus.MustNewConstMetric(e.backendServerEjections, prometheus.CounterValue, pool.ServerEjects, poolName)
		ch <- prometheus.MustNewConstMetric(e.forwardErrors, prometheus.CounterValue, pool.ForwardError, poolName)
		ch <- prometheus.MustNewConstMetric(e.fragments, prometheus.CounterValue, pool.Fragments, poolName)

		for serverName, server := range pool.Servers {
			ch <- prometheus.MustNewConstMetric(e.serverEOF, prometheus.CounterValue, server.ServerEOF, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.serverErr, prometheus.CounterValue, server.ServerErr, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.serverTimeouts, prometheus.CounterValue, server.ServerTimedout, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.activeServerConnections, prometheus.GaugeValue, server.ServerConnections, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.serverEjectedAt, prometheus.GaugeValue, server.ServerEjectedAt, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.requestCount, prometheus.CounterValue, server.Requests, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.requestBytes, prometheus.CounterValue, server.RequestBytes, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.responseCount, prometheus.CounterValue, server.Responses, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.responseBytes, prometheus.CounterValue, server.ResponseBytes, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.incomingQueueGauge, prometheus.GaugeValue, server.InQueue, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.incomingQueueBytesGauge, prometheus.GaugeValue, server.InQueueBytes, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.outgoingQueueGauge, prometheus.GaugeValue, server.OutQueue, poolName, serverName)
			ch <- prometheus.MustNewConstMetric(e.outgoingQueueBytesGauge, prometheus.GaugeValue, server.OutQueueBytes, poolName, serverName)
		}
	}
}

func (e *exporter) collect() (*stats, error) {
	// TODO: Reuse connection, connect with net.DialTCP?
	conn, err := net.DialTimeout("tcp", e.endpoint, e.timeout)
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
