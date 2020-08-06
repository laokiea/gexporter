// scrape metrics

package exporter

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	ticker          *time.Ticker
	stracePids      = make(map[int32]bool)
	CpuOb           = NewCpuOb()
	MemoryOb        = NewMemoryOb()
	gTimeChan       = make(chan int64, 0)
	gDown           int32 = 0
)

// collect entry
func CollectWorkLoadUsage() {
	timeUseGaugeVec := GetGaugeVec("scrape_time_use", "scrape time use", []string{})
	ticker = time.NewTicker(time.Second * time.Duration(gExporterConfig.getConfig("scrape_interval").(int)))
	for {
		select {
		case <- ticker.C:
			timeUseStart := time.Now().UnixNano() / 1e6
			// cpu usage
			go CpuOb.CalCpuUsage()
			// load average
			go CpuOb.LoadAverage()
			// uss memory usage
			go MemoryOb.ExposeUssMemoryUsage()
			// time use
			timeUseStop := <- gTimeChan
			timeUse := timeUseStop - timeUseStart
			timeUseGaugeVec.WithLabelValues().Set(float64(timeUse))
		}
	}
}

func timeUseCondition() {
	mtx := sync.Mutex{}
	atomic.AddInt32(&gDown, 1)
	mtx.Lock()
	if gDown == 3 {
		gTimeChan <- time.Now().UnixNano() / 1e6
	}
	mtx.Unlock()
}

// expose metrics to pushgateway
func NormalUsagePushGateway(indicator *Indicator) {

}
