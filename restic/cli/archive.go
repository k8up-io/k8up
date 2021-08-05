package cli

// Archive uploads the last version of each snapshot to S3.
func (r *Restic) Archive(restoreFilter string, verifyRestore bool, tags ArrayOpts) error {

	archiveLogger := r.logger.WithName("archive")

	err := r.LastSnapshots(tags)
	if err != nil {
		archiveLogger.Error(err, "could not list snapshots")
	}

	archiveLogger.Info("archiving latest snapshots for every host")

	for _, v := range r.snapshots {
		PVCname := r.parsePath(v.Paths)
		archiveLogger.Info("starting archival for", "namespace", v.Hostname, "pvc", PVCname)
		err := r.Restore(v.ID, RestoreOptions{RestoreType: S3Restore, RestoreFilter: restoreFilter, Verify: verifyRestore}, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
