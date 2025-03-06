package exporters

import (
	"time"

	"github.com/maddsua/pulse/storage"
)

func aggregateUptimeEntries(entries []storage.UptimeEntry, interval time.Duration) []storage.UptimeEntry {

	if len(entries) < 2 {
		return entries
	}

	//	todo: do the aggregation
}
