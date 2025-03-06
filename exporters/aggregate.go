package exporters

import (
	"time"

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
	//	todo: group data
}
