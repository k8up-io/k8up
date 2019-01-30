package restic

import "fmt"

// UnlockStruct holds the state of the unlock command.
type UnlockStruct struct {
	genericCommand
}

func newUnlock() *UnlockStruct {
	return &UnlockStruct{}
}

// Unlock removes stale locks. A lock is stale either if the pid isn't found on
// the current machine or if it's older than 30 min. (According to the restic
// source code)
func (u *UnlockStruct) Unlock(all bool) {
	fmt.Println("Removing locks...")
	args := []string{"unlock"}
	if all {
		args = append(args, "--remove-all")
	}
	u.genericCommand.exec(args, commandOptions{print: true})
}
