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
	commonProcessLabelNames = []string{"rank", "type"}
	processGaugeVecMetrics = NewGaugeVecMetrics("process_workload_usage", "Cpu and mem usage of per process", commonProcessLabelNames)
	collectors = make([]prometheus.Collector, 0)
	processGaugeVec = GetMetricsCollect()
	straceMetricsVec = GetStraceMetricsGaugeVec()
	usageGaugeVec = getUsageCounterVec()
	loadAverageHistogramVec = NewLoadAverageHistogramVec()
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
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: processGaugeVecMetrics.Name,
		Help: processGaugeVecMetrics.Help,
	}, processGaugeVecMetrics.LabelsName)
	collectors = append(collectors, vec)
	return vec
}

func GetStraceMetricsGaugeVec() *prometheus.GaugeVec {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "strace_metrics",
		Help: "strace command return",
	}, []string{"pid", "command", "call_name"})
	collectors = append(collectors, vec)
	return vec
}

func getUsageCounterVec() *prometheus.GaugeVec {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "workload_usage_gauge",
		Help: "memory and cpu usage gauge",
	}, []string{"type", "subtype"})
	collectors = append(collectors, vec)
	return vec
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

func NewLoadAverageHistogramVec() *prometheus.HistogramVec {
	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "load_average",
		Help: "load average",
		Buckets: CpuOb.GetLoadAverageBucket(),
	}, []string{"range"})
	collectors = append(collectors, vec)
	return vec
}

func GetGaugeVec(name string, help string, labels []string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labels)
	prometheus.MustRegister(gaugeVec)
	return gaugeVec
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