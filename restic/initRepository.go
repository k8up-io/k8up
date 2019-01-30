package restic

import (
	"fmt"

	"git.vshn.net/vshn/wrestic/s3"
)

// Initrepo checks if there's a repository and initializes it.
type Initrepo struct {
	genericCommand
}

func newInitrepo() *Initrepo {
	return &Initrepo{}
}

// InitRepository checks if there's a repository and initializes it. It expects
// a working
func (i *Initrepo) InitRepository(s3Client *s3.Client) {
	_, err := s3Client.Stat("config")
	if err != nil {
		fmt.Println("No repository available, initialising...")
		args := []string{"init"}
		i.genericCommand.exec(args, commandOptions{print: true})
	}
}
