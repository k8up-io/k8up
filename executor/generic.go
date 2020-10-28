package executor

import (
	"github.com/go-logr/logr"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/queue"
)

type generic struct {
	job.Config
}

func (g *generic) Logger() logr.Logger {
	return g.Log
}

func (*generic) Exclusive() bool {
	return false
}

// func (g *generic) GetName() string {
// 	return g.Name
// }

func (g *generic) GetRepository() string {
	return g.Repository
}

func NewExecutor(obj job.Object, config job.Config) queue.Executor {
	switch obj.GetType() {
	case "backup":
		return NewBackupExecutor(config)
	case "check":
		return NewCheckExecutor(config)
	}
	return nil
}
