package queue

import (
	"container/heap"
)

type QueuedJob struct {
	//TBD job item
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

func (pq *PriorityQueue) add(q *QueuedJob) {
	heap.Push(pq, q)
}

func (pq *PriorityQueue) get() *QueuedJob {
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
