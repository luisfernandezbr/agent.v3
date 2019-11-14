package memorylogs

import (
	"context"
	"runtime"
	"time"

	"github.com/hashicorp/go-hclog"
)

func Start(ctx context.Context, logger hclog.Logger, period time.Duration) {
	globalStart := time.Now()
	allocatedMB := getAllocatedMB()
	ticker := time.NewTicker(period)

	log := func() {
		allocatedMB = getAllocatedMB()
		timeSinceStartMin := int(time.Since(globalStart).Minutes())

		logger.Info("memory use", "MB", allocatedMB, "min", timeSinceStartMin)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log()
			}
		}
	}()
}

func getAllocatedMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.HeapAlloc / 1024 / 1024)
}
