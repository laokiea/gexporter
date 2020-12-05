package exporter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	// log "github.com/sirupsen/logrus"
	logtax "log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	SmemCommandNotInstalledErr = "smem command not installed"
)

// Normal indicator include cpu/mem usage
type Indicator struct {
	UssMemUsage 	float64  `json:"uss_mem_usage"`
	PssMemUsage 	float64  `json:"pss_mem_usage"`
	RssMemUsage 	float64  `json:"rss_mem_usage"`
	Pid             int32    `json:"pid"`
	Command         string   `json:"command"`
}

// memory info struct
type MemoryInfo struct {
	RssMemUsage                float64
	UssMemUsage                float64
	PssMemUsage                float64
	MemIndicators     	   []*Indicator
}

// Strace metrics
type StraceMetrics struct {
	I           *Indicator
	Seconds  	string   `json:"total_seconds"`
	Calls       float64  `json:"call_times"`
	Syscall     string   `json:"syscall_name"`
}

func NewMemoryOb() *MemoryInfo {
	MI := MemoryInfo{}
	MI.MemIndicators = make([]*Indicator, 0)

	return &MI
}

func (memory *MemoryInfo) ExposeUssMemoryUsage() {
	defer timeUseCondition()
	memory.GetMemoryIndicators()
	// total memory usage
	memory.exposePssTotalMemUsage()
	// top10 memory usage
	exporter := gExporterConfig.Configs["exporter"].(string)
	for rank,indicator := range memory.MemIndicators[:10] {
		switch exporter {
		case "pushgateway":
			//NormalUsagePushGateway(indicator)
		case "expose":
			memory.exposeNormalUssUsage(indicator, strconv.FormatInt(int64(rank), 10))
		}
	}
	// reset
	memory.resetMemoryUsage()
}

// fix command name
//
func (memory *MemoryInfo) fixCommandName(indicator *Indicator) {
	name := indicator.Command
	if strings.Contains(name, string(os.PathSeparator)) {
		lastSlashPos := strings.LastIndex(name, string(os.PathSeparator))
		indicator.Command = name[lastSlashPos+1:]
	}
	indicator.Command = fmt.Sprintf("%s,%d", indicator.Command, indicator.Pid)
}

// uss memory usage expose
func (memory *MemoryInfo) exposeNormalUssUsage(indicator *Indicator, rank string) {
	processGaugeVec.With(prometheus.Labels{
		//"command" : indicator.Command,
		"rank": rank,
		//"pid" : strconv.FormatInt(int64(indicator.Pid), 10),
		"type" : "mem",
	}).Set(indicator.UssMemUsage)
}

// expose total memory usage
func (memory *MemoryInfo) exposePssTotalMemUsage() {
	usageGaugeVec.With(prometheus.Labels{"type": "mem", "subtype": "mem"}).Set(memory.PssMemUsage)
}

// reset memory info obj memory usage
func (memory *MemoryInfo) resetMemoryUsage() {
	memory.UssMemUsage = 0.0
	memory.PssMemUsage = 0.0
	memory.RssMemUsage = 0.0
	memory.MemIndicators = memory.MemIndicators[:0]
}

// calculate pss memory usage
func (memory *MemoryInfo) CalPssMemoryUsage() {
	for _,ussIndicator := range memory.MemIndicators {
		memory.PssMemUsage += ussIndicator.PssMemUsage
		memory.UssMemUsage += ussIndicator.UssMemUsage
	}
}

// get uss memory usage indicators
func (memory *MemoryInfo) GetMemoryIndicators() {
	if !memory.checkSmemCommandInstalled() {
		//log.WithFields(log.Fields{"skip":7}).Fatal(errors.New(SmemCommandNotInstalledErr))
		logtax.Fatal(errors.New(SmemCommandNotInstalledErr))
	}

	cmd := `smem -s pss -rHp -c "pid uss pss command" | head -n %d | awk '{if(NR > 0) print "{\"uss_mem_usage\":" $2 ",\"pss_mem_usage\":" $3 ",\"command\":\""} {for (i=4;i<=NF;i++)printf("%s ", $i);}  {print "\",\"pid\":" $1 "}"}'`
	result,err := exec.Command("sh", "-c", fmt.Sprintf(cmd, gExporterConfig.Configs["max_process_num"].(int), "%s")).Output()
	if err != nil {
		//log.WithFields(log.Fields{"skip":7}).Fatal(err.Error())
		logtax.Fatal(err.Error())
	}

	metricsString := strings.Trim(string(result), "\n")
	// trim \n
	metricsString = strings.ReplaceAll(metricsString, "%", "")
	metricsString = strings.ReplaceAll(metricsString, "\"command\":\"\n", "\"command\":\"")
	metricsSlice := strings.Split(metricsString, "\n")

	memory.MemIndicators = memory.MemIndicators[:0]
	for _,metric := range metricsSlice {
		var rssIndicator = Indicator{}
		metric = strings.ReplaceAll(metric, "\n", " ")
		if err := json.Unmarshal([]byte(metric), &rssIndicator);err != nil {
			//log.WithFields(log.Fields{"skip":7}).Error(err.Error())
			logtax.Println(err.Error())
			logtax.SetPrefix("[Info]")
			logtax.Println(string(result))
			continue
		}

		memory.fixCommandName(&rssIndicator)
		if rssIndicator.Command == excludeSelfProcess {
			continue
		}

		go memory.HighUsageCheck(&rssIndicator)
		memory.MemIndicators = append(memory.MemIndicators, &rssIndicator)
	}

	memory.CalPssMemoryUsage()
}

// high usage check
// use Uss
func (memory *MemoryInfo) HighUsageCheck(indicator *Indicator) {
	if indicator.UssMemUsage >= HighUsageMemThreshold {
		memory.CollectStraceMetrics(indicator)
	}
}

// strace process system call detail
func (memory *MemoryInfo) CollectStraceMetrics(indicator *Indicator) {
	if runtime.GOOS != TargetOs || os.Getuid() != 0 {
		//log.WithFields(log.Fields{"skip":7}).Fatal(errors.New("strace must run as root within linux os"))
		logtax.Fatal(errors.New("strace must run as root within linux os"))
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
	execCmd := exec.Command("sh", "-c", highUsageCCmd)
	execCmd.Stdout = straceBuffer


	if err := execCmd.Start();err != nil {
		//log.WithFields(log.Fields{"skip":7}).Fatal(err.Error()+",Command start failed")
		logtax.Fatal(err.Error()+",Command start failed")
	}

	go func(pid int) {
		var straceTimer = time.NewTimer(time.Second * time.Duration(StraceAttachTime))
		defer straceTimer.Stop()

		select {
		case <-straceTimer.C:
			if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
				// log.WithFields(log.Fields{"skip":7}).Error(err.Error() + ",send SIGINT error")
				logtax.Println(err.Error() + ",send SIGINT error")
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
					memory.exposeHighUsageStraceMetrics(&metricsS)
				}
			}
			lineIndex++
		}
	}
}

// expose metrics
func (memory *MemoryInfo) exposeHighUsageStraceMetrics(metric *StraceMetrics) {
	straceMetricsVec.With(prometheus.Labels{
		"pid" : strconv.FormatInt(int64(metric.I.Pid), 10),
		"command" : metric.I.Command,
		"call_name" : metric.Syscall,
	}).Set(metric.Calls)
}

// check smem command installed
func (memory *MemoryInfo) checkSmemCommandInstalled() bool {
	wi,_ := exec.Command(`whereis smem`).Output()
	if string(wi) == " " {
		return false
	}
	return true
}

// get rss memory usage
// do not use this function instead of using GetUssMemoryUsage
func (memory *MemoryInfo) GetRssMemoryUsage() {
	var (
		maxProcessNum = gExporterConfig.Configs["max_process_num"].(int)
		cmdFormat = `ps aux | sort -r -n -k 4 | head -n %d | awk '{if(NR > 0) print "{\"rss_mem_usage\":" $4 ",\"pid\":" $2 ",\"command\":\""} {if(NR > 0) for (i=11;i<=NF;i++)printf("%s ", $i);}  {if(NR > 0) print "\"}"}'`
		metricsCmd = fmt.Sprintf(cmdFormat, maxProcessNum + 1, "%s")
	)

	cmd := exec.Command("sh", "-c", metricsCmd)
	result, err := cmd.Output()
	if err != nil {
		//log.WithFields(log.Fields{"skip":7}).Fatal(err.Error())
		logtax.Fatal(err.Error())
	}

	metricsString := strings.Trim(string(result), "\n")
	// trim \n
	metricsString = strings.ReplaceAll(metricsString, "\"command\":\"\n", "\"command\":\"")
	metricsSlice := strings.Split(metricsString, "\n")

	for _,metric := range metricsSlice {
		var indicator = Indicator{}
		metric = strings.ReplaceAll(metric, "\n", " ")
		if err := json.Unmarshal([]byte(metric), &indicator);err != nil {
			// log.WithFields(log.Fields{"skip":7}).Error(err.Error())
			logtax.Println(err.Error())
			continue
		}

		memory.fixCommandName(&indicator)
		if indicator.Command == excludeSelfProcess {
			continue
		}
		memory.RssMemUsage += indicator.RssMemUsage
	}
}