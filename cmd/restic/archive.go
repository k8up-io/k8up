package main

import (
	"fmt"
	"sort"
)

func archiveJob() {
	fmt.Println("Archiving latest snapshots for every host")
	tmpSnaps, err := listSnapshots()
	snapshots := snapList(tmpSnaps)
	if err != nil {
		commandError = err
		return
	}

	sort.Sort(sort.Reverse(snapshots))

	snapMap := make(map[string]snapshot)
	for _, snap := range snapshots {
		if _, ok := snapMap[snap.Hostname]; !ok {
			snapMap[snap.Hostname] = snap
		}
	}

	for _, v := range snapMap {
		fmt.Printf("Archive running for %v\n", v.Hostname)
		restoreJob(v.ID, *restoreType)
	}

}
