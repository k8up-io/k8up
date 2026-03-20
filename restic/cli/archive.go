package cli

// Archive uploads the last version of each snapshot to S3.
func (r *Restic) Archive(options RestoreOptions, tags ArrayOpts, paths ArrayOpts) error {

	archiveLogger := r.logger.WithName("archive")

	err := r.LastSnapshots(tags, paths)
	if err != nil {
		archiveLogger.Error(err, "could not list snapshots")
	}

	archiveLogger.Info("archiving latest snapshots for every host")

	for _, v := range r.snapshots {
		PVCname := r.parsePath(v.Paths)
		archiveLogger.Info("starting archival for", "namespace", v.Hostname, "pvc", PVCname)
		err := r.Restore(v.ID, options, nil, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
