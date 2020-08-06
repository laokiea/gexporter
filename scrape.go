// scrape metrics

package exporter

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	ticker               *time.Ticker
	stracePids           = make(map[int32]bool)
	CpuOb                = NewCpuOb()
	MemoryOb             = NewMemoryOb()
	gTimeChan            = make(chan float64, 0)
	gCountDown int32     = 0
	gMtx                 = sync.Mutex{}
)

// collect entry
func CollectWorkLoadUsage() {
	timeUseGaugeVec := GetGaugeVec("scrape_time_use", "scrape time use", []string{})
	ticker = time.NewTicker(time.Second * time.Duration(gExporterConfig.getConfig("scrape_interval").(int)))
	for {
		select {
		case <- ticker.C:
			timeUseStart := float64(time.Now().UnixNano()) / 1e6
			// cpu usage
			go CpuOb.CalCpuUsage()
			// load average
			go CpuOb.LoadAverage()
			// uss memory usage
			go MemoryOb.ExposeUssMemoryUsage()
			// time use
			timeUseStop := <- gTimeChan
			timeUse := timeUseStop - timeUseStart
			timeUseGaugeVec.WithLabelValues().Set(timeUse)
		}
	}
}

func timeUseCondition() {
	atomic.AddInt32(&gCountDown, 1)
	gMtx.Lock()
	if gCountDown == 3 {
		gTimeChan <- float64(time.Now().UnixNano()) / 1e6
	}
	gMtx.Unlock()
}

// expose metrics to pushgateway
func NormalUsagePushGateway(indicator *Indicator) {

}
