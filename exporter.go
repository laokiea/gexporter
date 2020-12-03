// Prometheus exporter for exposing system processes metrics
// include memory/cpu usage and system calls statistics
// support pushgateway and expose via http server

package exporter

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"runtime"
	"strconv"
)

type Metrics struct {
	Name string
	Help string
}

type GExporterLogFormatter struct {}

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
	// init log
	log.SetFormatter(new(GExporterLogFormatter))
	// set logger output file
	_ = os.MkdirAll(LogDir, 0777)
	file, err := os.OpenFile(LogDir+"exporter.log", os.O_CREATE | os.O_WRONLY | os.O_APPEND, 0666)
	if err != nil {
		log.WithFields(log.Fields{"skip":5}).Fatal(err)
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

func (f *GExporterLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	skip, e := entry.Data["skip"]
	if e {
		_, file, line, ok := runtime.Caller(skip.(int))
		if !ok {
			file = "???"
			line = 0
		}

		delete(entry.Data, "skip")
		entry.Data["file"] = file
		entry.Data["line"] = line
	}

	entry.Data["level"] = entry.Level
	entry.Data["message"] = entry.Message

	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
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
		Addr: "0.0.0.0:" + strconv.FormatInt(int64(gExporterConfig.Configs["prom_http_port"].(int)), 10),
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.WithFields(log.Fields{"skip":5}).Error(err.Error())
	}
}