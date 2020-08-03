// scrape metrics

package exporter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	ticker          *time.Ticker
	stracePids      = make(map[int32]bool)
	memUsageCount   float64
	CpuOb           = NewCpuOb()
	//cpuUsageCount   float64
	//cpuUsageType = []string{"user", "nice", "system", "idle", "iowait", "irq", "softirq", "steal", "guest"}
)

// Normal indicator include cpu/mem usage
type Indicator struct {
	CpuUsage 	float64  `json:"cpu_usage"`
	MemUsage 	float64  `json:"mem_usage"`
	Pid         int32    `json:"pid"`
	Command     string   `json:"command"`
}

// Strace metrics
type StraceMetrics struct {
	I           *Indicator
	Seconds  	string   `json:"total_seconds"`
	Calls       float64  `json:"call_times"`
	Syscall     string   `json:"syscall_name"`
}

// collect entry
func CollectWorkLoadUsage() {
	ticker = time.NewTicker(time.Second * time.Duration(gExporterConfig.getConfig("scrape_interval").(int)))
	for {
		select {
		case <- ticker.C:
			indicators, err := CollectNormalIndicators()
			if err != nil {
				log.Error(err.Error())
				continue
			}

			// counter
			usageGaugeVec.With(prometheus.Labels{"type": "mem", "subtype": "mem"}).Set(memUsageCount)
			memUsageCount = 0.0
			// cpu usage
			CpuOb.CalCpuUsage()
			// load average
			CpuOb.LoadAverage()

			for _,indicator := range indicators[:10] {
				exporter := gExporterConfig.Configs["exporter"].(string)
				switch exporter {
				case "pushgateway":
					NormalUsagePushGateway(indicator)
				case "expose":
					NormalUsageExpose(indicator)
				}
			}
		}
	}
}

// process normal indicators collect
func CollectNormalIndicators() (indicators []*Indicator, err error) {
	var (
		maxProcessNum = gExporterConfig.Configs["max_process_num"].(int)
		cmdFormat = `ps aux | sort -r -n -k 4 | head -n %d | awk '{if(NR > 0) print "{\"cpu_usage\":" $3 ",\"mem_usage\":" $4 ",\"pid\":" $2 ",\"command\":\""} {if(NR > 0) for (i=11;i<=NF;i++)printf("%s ", $i);}  {if(NR > 0) print "\"}"}'`
		metricsCmd = fmt.Sprintf(cmdFormat, maxProcessNum + 1, "%s")
	)

	cmd := exec.Command("bash", "-c", metricsCmd)
	result, err := cmd.Output()
	if err != nil {
		log.Error(err.Error())
		return
	}

	metricsString := strings.Trim(string(result), "\n")
	// trim \n
	metricsString = strings.ReplaceAll(metricsString, "\"command\":\"\n", "\"command\":\"")
	metricsSlice := strings.Split(metricsString, "\n")
	indicators = make([]*Indicator, 0, len(metricsSlice))
	indicators = indicators[:0]

	for _,metric := range metricsSlice {
		var indicator = Indicator{}
		metric = strings.ReplaceAll(metric, "\n", " ")
		if err := json.Unmarshal([]byte(metric), &indicator);err != nil {
			log.Error(err.Error())
			continue
		}

		fixCommandName(&indicator)
		if indicator.Command == excludeSelfProcess {
			continue
		}

		go HighUsageCheck(&indicator)

		// counter
		memUsageCount += indicator.MemUsage
		indicators = append(indicators, &indicator)
	}

	return
}

// strace process system call detail
func CollectStraceMetrics(indicator *Indicator) {
	if runtime.GOOS != TargetOs || os.Getuid() != 0 {
		log.Fatal(errors.New("strace must run as root within linux os"))
		return
	}

	var (
		lineIndex       int
		metricsSlice    []string
		straceFileName  = fmt.Sprintf(StraceOutputFile, indicator.Pid)
		straceFile, _   = os.OpenFile(straceFileName, os.O_CREATE | os.O_RDWR | os.O_APPEND, 0666)
		straceBytes     = make([]byte, 0)
		straceBuffer    = &bytes.Buffer{}
	)

	if stracePids[indicator.Pid] == true {
		return
	} else {
		stracePids[indicator.Pid] = true
	}

	defer straceFile.Close()

	highUsageCCmd := fmt.Sprintf("strace -u work -f -p %d -c -e trace=all -o %s", indicator.Pid, straceFileName)
	execCmd := exec.Command("bash", "-c", highUsageCCmd)
	execCmd.Stdout = straceBuffer


	if err := execCmd.Start();err != nil {
		log.Fatal(err.Error()+",Command start failed")
	}

	go func(pid int) {
		var straceTimer = time.NewTimer(time.Second * time.Duration(StraceAttachTime))
		defer straceTimer.Stop()

		select {
		case <-straceTimer.C:
			if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
				log.Error(err.Error() + ",send SIGINT error")
			}
			return
		}
	}(execCmd.Process.Pid)

	_ = execCmd.Wait()

	readN, _  := straceBuffer.Read(straceBytes)

	if readN == 0 {
		bufReader := bufio.NewScanner(straceFile)
		bufReader.Split(bufio.ScanLines)
		for bufReader.Scan() {
			lineText := bufReader.Text()
			if lineIndex > 1 {
				lineText = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(lineText), " ")
				metricsSlice = strings.Split(lineText, " ")
				if metricsSlice[0] == straceOutputEnd {
					break
				}

				calls,_ := strconv.ParseFloat(metricsSlice[3], 64)
				metricsS := StraceMetrics{
					I: indicator,
					Seconds: metricsSlice[1],
					Calls: calls,
				}
				if len(metricsSlice) == 6 {
					metricsS.Syscall = metricsSlice[5]
				} else {
					metricsS.Syscall = metricsSlice[4]
				}

				switch gExporterConfig.Configs["exporter"].(string) {
				case "expose":
					exposeHighUsageStraceMetrics(&metricsS)
				}
			}
			lineIndex++
		}
	}
}

// high usage check
func HighUsageCheck(indicator *Indicator) {
	var (
		cpuUsage = indicator.CpuUsage
		memUsage = indicator.MemUsage
	)

	if cpuUsage >= HighUsageCpuThreshold || memUsage >= HighUsageMemThreshold {
		// expose
		CollectStraceMetrics(indicator)
	}
}

func NormalUsageExpose(indicator *Indicator) {
	processGaugeVec.With(prometheus.Labels{
		"command" : indicator.Command,
		//"pid" : strconv.FormatInt(int64(indicator.Pid), 10),
		"type" : "cpu",
	}).Set(indicator.CpuUsage)

	processGaugeVec.With(prometheus.Labels{
		"command" : indicator.Command,
		//"pid" : strconv.FormatInt(int64(indicator.Pid), 10),
		"type" : "mem",
	}).Set(indicator.MemUsage)
}

// expose metrics to pushgateway
func NormalUsagePushGateway(indicator *Indicator) {

}

// expose metrics
func exposeHighUsageStraceMetrics(metric *StraceMetrics) {
	straceMetricsVec.With(prometheus.Labels{
		"pid" : strconv.FormatInt(int64(metric.I.Pid), 10),
		"command" : metric.I.Command,
		"call_name" : metric.Syscall,
	}).Set(metric.Calls)
}

// fix command name
//
func fixCommandName(indicator *Indicator) {
	//name := indicator.Command
	//if strings.Contains(name, string(os.PathSeparator)) {
	//	lastSlashPos := strings.LastIndex(name, string(os.PathSeparator))
	//	indicator.Command = name[lastSlashPos+1:]
	//}
	indicator.Command = fmt.Sprintf("%s,%d", indicator.Command, indicator.Pid)
}
