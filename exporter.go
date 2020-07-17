// Prometheus exporter for exposing system processes metrics
// include memory/cpu usage and system calls statistics
// support pushgateway and expose via http server

package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

type Metrics struct {
	Name string
	Help string
}

type GaugeVecMetrics struct {
	*Metrics
	LabelsName []string
}

var (
	LogDir = "/data/logs/"
	gExporterConfig  = NewExporterConfig()
	commonProcessLabelNames = []string{"command", "pid", "type"}
	processGaugeVecMetrics = NewGaugeVecMetrics("process_workload_usage", "Cpu and mem usage of per process", commonProcessLabelNames)
	processGaugeVec = GetMetricsCollect()
	straceMetricsVec = GetStraceMetricsGaugeVec()
	collectors = []prometheus.Collector{processGaugeVec, straceMetricsVec}
)

func init() {
	// set logger output file
	file, err := os.OpenFile(LogDir+"exporter.log", os.O_CREATE | os.O_WRONLY | os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.SetOutput(file)

	// must register collector before expose/push
	prometheus.MustRegister(collectors...)

	// start expose http server
	if gExporterConfig.Configs["exporter"].(string) == "expose" {
		go PromHttpServerStart()
	}
}

func GetMetricsCollect() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: processGaugeVecMetrics.Name,
		Help: processGaugeVecMetrics.Help,
	}, processGaugeVecMetrics.LabelsName)
}

func GetStraceMetricsGaugeVec() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "strace_metrics",
		Help: "strace command return",
	}, []string{"pid", "command", "call_name"})
}

func NewGaugeVecMetrics(metricsName string, MetricsHelp string, labelNames []string) *GaugeVecMetrics {
	return &GaugeVecMetrics{
		&Metrics{
			Name: metricsName,
			Help: MetricsHelp,
		},
		labelNames,
	}
}

// a http server for exposing metrics
func PromHttpServerStart() {
	mux := http.NewServeMux()
	mux.Handle(MetricsHttpPath, promhttp.Handler())

	httpServer := &http.Server{
		Handler: mux,
		Addr: "0.0.0.0:" + MetricsHttpPort,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.Error(err.Error())
	}
}