package exporters

import (
	"time"

	"github.com/guregu/null"
	"github.com/maddsua/pulse/storage"
)

func aggregateUptimeEntries(entries []storage.UptimeEntry, interval time.Duration) []storage.UptimeEntry {

	if len(entries) < 2 {
		return entries
	}

	var result []storage.UptimeEntry

	var group []storage.UptimeEntry
	groupTime := entries[0].Time

	for _, entry := range entries {

		if entry.Time.Sub(groupTime) > interval {
			result = append(result, mergeLabeledUptimeEntries(group)...)
			group = []storage.UptimeEntry{}
			groupTime = entry.Time
		}

		group = append(group, entry)
	}

	if len(group) > 0 {
		result = append(result, mergeLabeledUptimeEntries(group)...)
	}

	return result
}

func mergeLabeledUptimeEntries(entries []storage.UptimeEntry) []storage.UptimeEntry {

	byLabel := map[string][]storage.UptimeEntry{}
	for _, entry := range entries {
		byLabel[entry.Label] = append(byLabel[entry.Label], entry)
	}

	var result []storage.UptimeEntry
	for _, labelEntries := range byLabel {
		result = append(result, mergeUptimeEntries(labelEntries))
	}

	return result
}

func mergeUptimeEntries(entries []storage.UptimeEntry) storage.UptimeEntry {

	var latencyAvg int
	var elapsedAvg time.Duration

	var maxStatusVal storage.ServiceStatus
	var maxStatusN int

	statuses := map[int]int{}

	var maxHttpStatusVal null.Int
	var maxHttpStatusN int

	httpStatuses := map[int]int{}

	for _, entry := range entries {

		if latencyAvg >= 0 {
			latencyAvg += entry.LatencyMs
		} else {
			latencyAvg = -1
		}

		elapsedAvg += entry.Elapsed

		if entry.HttpStatus.Valid {

			key := int(entry.HttpStatus.Int64)
			nextN := statuses[key] + 1
			statuses[key] = nextN

			if nextN > int(maxHttpStatusN) {
				maxHttpStatusN = nextN
				maxHttpStatusVal = entry.HttpStatus
			}
		}

		key := int(entry.Status)
		nextN := httpStatuses[key] + 1
		httpStatuses[key] = nextN

		if nextN > maxStatusN {
			maxStatusN = nextN
			maxStatusVal = entry.Status
		}
	}

	result := storage.UptimeEntry{
		LatencyMs:  latencyAvg,
		Elapsed:    (elapsedAvg / time.Duration(len(entries))),
		Status:     storage.ServiceStatus(maxStatusVal),
		HttpStatus: maxHttpStatusVal,
	}

	if latencyAvg >= 0 {
		result.LatencyMs = (latencyAvg / len(entries))
	}

	return result
}
