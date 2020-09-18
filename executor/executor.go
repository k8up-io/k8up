// executor will execute all jobs in the queue by priority.

package executor

type Executor interface {
	Execute() error
}

//TODO: go routine that fetches the newest jobs
