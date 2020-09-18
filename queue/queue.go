package queue

import (
	"container/heap"
	"sync"

	"github.com/vshn/k8up/executor"
)

var (
	Queue = newPriorityQueue()
	wg    = sync.WaitGroup{}
)

//TODO: how to handle the prio
type QueuedJob struct {
	Job      executor.Executor
	priority int
	index    int
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*QueuedJob

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	q := x.(*QueuedJob)
	q.index = n
	*pq = append(*pq, q)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Add(q *QueuedJob) {
	wg.Add(1)
	heap.Push(pq, q)
	wg.Done()
}

func (pq *PriorityQueue) Get() *QueuedJob {
	wg.Add(1)
	defer wg.Done()
	return heap.Pop(pq).(*QueuedJob)
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(q *QueuedJob, priority int) {
	q.priority = priority
	heap.Fix(pq, q.index)
}

// newPriorityQueue returns a new, empty queue.
func newPriorityQueue() *PriorityQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	return &pq
}
