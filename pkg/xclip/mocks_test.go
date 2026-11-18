package xclip

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

type MockExecer struct {
	mu         sync.Mutex
	calls      map[string][][]byte
	unexpected []string
	t          *testing.T
}

func NewMockExecer(t *testing.T) *MockExecer {
	return &MockExecer{
		calls: make(map[string][][]byte),
		t:     t,
	}
}

func (m *MockExecer) AddCall(args string, output []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[args] = append(m.calls[args], output)
}

func (m *MockExecer) AddCallN(args string, output []byte, n int) {
	for i := 0; i < n; i++ {
		m.AddCall(args, output)
	}
}

// Unexpected returns the calls that had no registered output.
func (m *MockExecer) Unexpected() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.unexpected...)
}

// Run returns an error for unexpected calls instead of failing the test,
// because it may be invoked from a background goroutine (e.g. Watch) where
// t.Fatalf is not allowed.
func (m *MockExecer) Run(cmd *exec.Cmd) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := strings.Join(cmd.Args, " ")
	v, ok := m.calls[args]
	if !ok {
		m.unexpected = append(m.unexpected, args)
		return fmt.Errorf("unexpected call: %s", args)
	}
	cmd.Stdout.Write(v[0])
	v = v[1:]
	if len(v) > 0 {
		m.calls[args] = v
	} else {
		delete(m.calls, args)
	}
	return nil
}
