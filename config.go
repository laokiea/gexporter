// collect metrics configure

package exporter

import (
	"errors"
	"flag"
)

const (
	DefaultScrapeInterval 	= 10
	DefaultExporter       	= "expose"
	MaxCollectProcessNum  	= 50
	HighUsageCpuThreshold   = 40.0
	HighUsageMemThreshold   = 50.0
	MetricsHttpPath       	= "/metrics"
	MetricsHttpPort       	= "80"
	TargetOs              	= "linux"
	StraceAttachTime      	= 5
	StraceOutputFile      	= "/data/logs/exporter_strace_%d.log"
	straceOutputEnd         = "------"
	excludeSelfProcess      = "gexporter_main"
)

type Config interface {
	parseConfig()
	getConfig()   *ConfigValues
}

type ConfigValues map[string]interface{}

type GExporterConfig struct {
	Configs ConfigValues
}

var (
	configNames = []string{"exporter", "scrape_interval", "max_process_num"}
)

func NewExporterConfig() *GExporterConfig {
	gec := &GExporterConfig{
		Configs: make(ConfigValues),
	}
	gec.parseConfig()

	return gec
}

func (config *GExporterConfig) parseSingle(configName string) {
	switch configName {
	case "exporter":
		exporter := flag.String("exporter", DefaultExporter, "exporter fashion")
		if *exporter != "pushgateway" && *exporter != "expose" {
			panic(errors.New("unsupport exporter"))
		} else {
			config.Configs["exporter"] = *exporter
		}
	case "scrape_interval":
		scrapeInterval := flag.Int("scrape-interval", DefaultScrapeInterval, "scraping interval")
		if *scrapeInterval > 60 || *scrapeInterval < 1 {
			panic(errors.New("scrape interval over limit"))
		} else {
			config.Configs["scrape_interval"] = *scrapeInterval
		}
	case "max_process_num":
		maxProcessNum := flag.Int("max-process-num", MaxCollectProcessNum, "max process num")
		if *maxProcessNum > 200 {
			panic(errors.New("max process num over limit"))
		} else {
			config.Configs["max_process_num"] = *maxProcessNum
		}
	}
}

func (config *GExporterConfig) parseConfig() {
	for _,name := range configNames {
		config.parseSingle(name)
	}
}

func (config *GExporterConfig) getConfig(configName string) interface{} {
	return config.Configs[configName]
}