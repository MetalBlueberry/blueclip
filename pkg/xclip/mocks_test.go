package xclip

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

type MockExecer struct {
	calls map[string][][]byte
	t     *testing.T
}

func NewMockExecer(t *testing.T) *MockExecer {
	return &MockExecer{
		calls: make(map[string][][]byte),
		t:     t,
	}
}

func (m *MockExecer) AddCall(args string, output []byte) {
	m.calls[args] = append(m.calls[args], output)
}

func (m *MockExecer) AddCallN(args string, output []byte, n int) {
	for i := 0; i < n; i++ {
		m.calls[args] = append(m.calls[args], output)
	}
}

func (m *MockExecer) Run(cmd *exec.Cmd) error {
	args := strings.Join(cmd.Args, " ")
	v, ok := m.calls[args]
	if !ok {
		m.t.Fatalf("Run() called with unexpected args: %s", args)
		return fmt.Errorf("Run() called with unexpected args: %s", args)
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
