package main

import (
	"fmt"
	"sort"
)

func archiveJob() error {
	fmt.Println("Archiving latest snapshots for every host")
	tmpSnaps, err := listSnapshots()
	if err != nil {
		commandError = err
		return err
	}
	snapshots := snapList(tmpSnaps)

	sort.Sort(sort.Reverse(snapshots))

	snapMap := make(map[string]snapshot)
	for _, snap := range snapshots {
		if _, ok := snapMap[snap.Hostname]; !ok {
			snapMap[snap.Hostname] = snap
		}
	}

	for _, v := range snapMap {
		fmt.Printf("Archive running for %v\n", v.Hostname)
		if err = restoreJob(v.ID, *restoreType); err != nil {
			return err
		}
	}
	return nil
}
