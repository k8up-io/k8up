package queue

import (
	"errors"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/api/v1"

	"github.com/go-logr/logr"
)

type mockExecutor struct {
	exclusive  bool
	repository string
}

func (m *mockExecutor) Execute() error {
	return errors.New("not implemented")
}
func (m *mockExecutor) Exclusive() bool {
	return m.exclusive
}
func (m *mockExecutor) Logger() logr.Logger {
	return logr.Discard()
}
func (m *mockExecutor) GetRepository() string {
	return m.repository
}
func (m *mockExecutor) GetJobNamespace() string {
	return "test"
}
func (m *mockExecutor) GetJobType() k8upv1.JobType {
	return k8upv1.ArchiveType
}
func (m *mockExecutor) GetConcurrencyLimit() int {
	return 1
}

func TestExecutionQueue(t *testing.T) {
	q := newExecutionQueue()

	if !q.IsEmpty("repo1") || !q.IsEmpty("repo2") || !q.IsEmpty("") {
		t.Fatal("queue is supposed to be empty")
	}

	m1 := &mockExecutor{false, "repo1"}
	q.Add(m1)
	m2 := &mockExecutor{true, "repo1"}
	q.Add(m2)
	m3 := &mockExecutor{true, "repo2"}
	q.Add(m3)

	a1 := q.Get("repo1")
	a2 := q.Get("repo1")
	a3 := q.Get("repo2")

	if a1 != m2 {
		t.Error("expected to retrieve exclusive executor first")
	}
	if a2 != m1 {
		t.Error("expected to retrieve non-exclusive executor second")
	}
	if a3 != m3 {
		t.Error("expected to retrieve repo2")
	}
}
