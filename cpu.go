package exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	cpuUsageType = []string{"user", "nice", "system", "idle", "iowait", "irq", "softirq", "steal", "guest"}
	loadAverageWorkConstant float64 = 3.0
)

// cpu info struct
type CpuInfo struct {
	PhysicalCpuNum           uint8  // 物理处理器数量
	ModelName   		     string // 处理器型号
	CoresNum    		     uint8  // 核数量
	SiblingsNum 		     uint8  // 逻辑处理器数量
	SupportHT                bool   // support HT
	VirtualAddressSize       uint64  // virtual memory space size
	CpuCacheSize             uint64 // cpu level cache size
}

type CpuStat struct {
	*CpuInfo
	ProcessorId  		uint8
}

func (cpu *CpuInfo) GetLoadAverageBucket() (buckets []float64) {
	buckets = make([]float64, 0)
	// bucket range
	buckets = append(buckets, cpu.PCpuNumfloat64() / 40.0)
	buckets = append(buckets, cpu.PCpuNumfloat64() / 20.0)
	buckets = append(buckets, cpu.PCpuNumfloat64() / 10.0)
	buckets = append(buckets, cpu.PCpuNumfloat64() / 5.0)
	buckets = append(buckets, cpu.PCpuNumfloat64() / 4.0)
	buckets = append(buckets, cpu.PCpuNumfloat64() / 2.0)
	buckets = append(buckets, cpu.PCpuNumfloat64())
	buckets = append(buckets, cpu.PCpuNumfloat64() * loadAverageWorkConstant)

	return
}

// physical cpu num convert to float64
func (cpu *CpuInfo) PCpuNumfloat64() float64 {
	return float64(cpu.PhysicalCpuNum)
}

// observe load average
func (cpu *CpuInfo) LoadAverage() {
	defer timeUseCondition()
	load := cpu.getLoadAverage()
	for r,v := range load {
		loadAverageHistogramVec.WithLabelValues(r).Observe(v)
	}
}

// get load average
func (cpu *CpuInfo) getLoadAverage() (loadAverage map[string]float64) {
	loadAverage = make(map[string]float64)
	statistic,_ := exec.Command("bash", "-c", `uptime | grep -o -E 'load average:(.*)' | awk '{print $3 $4 $5}'`).Output()
	result := strings.Split(strings.Trim(string(statistic), "\n"), ",")
	for k,r := range []string{"1", "5", "15"} {
		loadAverage[r],_ = strconv.ParseFloat(result[k], 64)
	}

	return
}

// calculate cpu usage within 100% percent
func (cpu *CpuInfo) CalCpuUsage() {
	defer timeUseCondition()
	dataSample := make([]map[string]float64, 0)
	wg := sync.WaitGroup{}

	for i := 0;i < 2;i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dataSample = append(dataSample, cpu.getCpuStatDetail(1))
		}(i)
		time.Sleep(time.Millisecond * 2000)
	}
	wg.Wait()

	for _,f := range cpuUsageType {
		usageGaugeVec.With(prometheus.Labels{"type": "cpu", "subtype": f}).Set((dataSample[0][f] - dataSample[1][f]) / (dataSample[0]["total"] - dataSample[1]["total"]))
	}

	usageGaugeVec.With(prometheus.Labels{"type": "cpu", "subtype": "total"}).Set(1 - ((dataSample[0]["idle"] - dataSample[1]["idle"]) / (dataSample[0]["total"] - dataSample[1]["total"])))
}

// get cpu usage detail
func (cpu *CpuInfo) getCpuStatDetail(line int) (detail map[string]float64) {
	detail = make(map[string]float64)
	cpuStatCmd := exec.Command("bash", "-c", fmt.Sprintf("cat /proc/stat | awk 'NR==%d {$1=null;print $0}'", line))
	o,_ := cpuStatCmd.Output()
	cpuStat := regexp.MustCompile(`\s+`).ReplaceAllString(string(o), " ")
	cpuStatSlice := strings.Split(cpuStat, " ")

	var (
		i = 1
		total float64
	)
	for _,f := range cpuUsageType {
		detail[f],_ = strconv.ParseFloat(cpuStatSlice[i], 64)
		total += detail[f]
		i++
	}
	detail["total"] = total

	return
}

// expose physical cpu num
func (cpu *CpuInfo) ExposePCNum() {
	pcnGaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "physical_cpu_num",
		Help: "physical cpu num",
	}, []string{})
	prometheus.MustRegister(pcnGaugeVec)
	pcnGaugeVec.WithLabelValues().Set(cpu.PCpuNumfloat64())
}

// return new cpu obj
func NewCpuOb() *CpuInfo {
	CI := CpuInfo{}
	cpuInfo,_ := exec.Command("bash", "-c", "cat /proc/cpuinfo").Output()

	CI.PhysicalCpuNum = uint8(strings.Count(string(cpuInfo), "physical id"))
	CI.SiblingsNum = uint8(strings.Count(string(cpuInfo), "siblings"))
	CI.CoresNum = uint8(strings.Count(string(cpuInfo), "core id"))
	CI.ModelName = string(regexp.MustCompile(`model name\s+:\s(\w+)`).FindSubmatch(cpuInfo)[1])
	CI.CpuCacheSize,_ = strconv.ParseUint(string(regexp.MustCompile(`cache size\s+:\s(\d+)`).FindSubmatch(cpuInfo)[1]), 10, 64)
	CI.VirtualAddressSize,_ = strconv.ParseUint(string(regexp.MustCompile(`address sizes\s+:\s.+?(\d+)`).FindSubmatch(cpuInfo)[1]), 10, 64)
	CI.SupportHT = CI.SiblingsNum == CI.CoresNum

	CI.ExposePCNum()

	return &CI
}