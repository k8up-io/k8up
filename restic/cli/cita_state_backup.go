package cli

import (
	"fmt"
	"k8s.io/utils/exec"
)

func (r *Restic) DoCITAStateBackup(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string) error {
	execer := exec.New()
	err := execer.Command("cloud-op", "state-backup", fmt.Sprintf("%d", blockHeight),
		"--node-root", nodeRoot,
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--backup-path", backupPath,
		"--crypto", crypto,
		"--consensus", consensus).Run()
	if err != nil {
		return err
	}
	return nil
}

func (r *Restic) DoCITAStateRecover(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string) error {
	execer := exec.New()
	err := execer.Command("cloud-op", "state-recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", nodeRoot,
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--backup-path", backupPath,
		"--crypto", crypto,
		"--consensus", consensus).Run()
	if err != nil {
		return err
	}
	return nil
}
