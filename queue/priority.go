package queue

import (
	"container/heap"
)

// QueuedJob is the priority queue's internal object.
type QueuedJob struct {
	Job      Executor
	priority int
	index    int
}

// priorityQueue implements heap.Interface and holds Items.
type priorityQueue []*QueuedJob

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	q := x.(*QueuedJob)
	q.index = n
	*pq = append(*pq, q)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// Add will add a new job to the queue. It determines if the job is exclusive
// and set the priority accordingly.
func (pq *priorityQueue) add(e Executor) {
	q := &QueuedJob{
		Job: e,
	}
	if e.Exclusive() {
		q.priority = 1
	} else {
		q.priority = 2
	}
	heap.Push(pq, q)
}

// Get will get the next job of the queue and also remove the item.
func (pq *priorityQueue) get() Executor {
	getJob := heap.Pop(pq).(*QueuedJob)
	return getJob.Job
}

// newPriorityQueue returns a new, empty queue.
func newPriorityQueue() *priorityQueue {
	pq := make(priorityQueue, 0)
	heap.Init(&pq)
	return &pq
}
