package queue

import (
	"errors"
	"testing"

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
	return nil
}
func (m *mockExecutor) GetRepository() string {
	return m.repository
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
