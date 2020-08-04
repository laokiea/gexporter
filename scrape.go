// scrape metrics

package exporter

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	ticker          *time.Ticker
	stracePids      = make(map[int32]bool)
	CpuOb           = NewCpuOb()
	MemoryOb        = NewMemoryOb()
)

// collect entry
func CollectWorkLoadUsage() {
	ticker = time.NewTicker(time.Second * time.Duration(gExporterConfig.getConfig("scrape_interval").(int)))
	for {
		select {
		case <- ticker.C:
			// cpu usage
			CpuOb.CalCpuUsage()
			// load average
			CpuOb.LoadAverage()
			// uss memory usage
			MemoryOb.ExposeUssMemoryUsage()
		}
	}
}

// expose metrics to pushgateway
func NormalUsagePushGateway(indicator *Indicator) {

}

// fix command name
//
func fixCommandName(indicator *Indicator) {
	name := indicator.Command
	if strings.Contains(name, string(os.PathSeparator)) {
		lastSlashPos := strings.LastIndex(name, string(os.PathSeparator))
		indicator.Command = name[lastSlashPos+1:]
	}
	indicator.Command = fmt.Sprintf("%s,%d", indicator.Command, indicator.Pid)
}
