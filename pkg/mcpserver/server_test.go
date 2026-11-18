package mcpserver

import (
	"blueclip/pkg/db"
	"blueclip/pkg/service"
	"blueclip/pkg/xclip"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// connect starts a blueclip server on a temporary socket, with xclip faked so
// the real clipboard is never touched, and returns an MCP session to it.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	previous := xclip.Cli.RunFn
	xclip.Cli.RunFn = func(*exec.Cmd) error { return nil }
	t.Cleanup(func() { xclip.Cli.RunFn = previous })

	dir := t.TempDir()
	sock := filepath.Join(dir, "blueclip.sock")
	svc := service.NewService(&db.FileDB{Path: filepath.Join(dir, "history.bin")}, sock)
	server, err := service.NewServer(svc.Handler(), sock)
	require.NoError(t, err)
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	t.Cleanup(func() {
		server.Close()
		<-errCh
	})
	require.Eventually(t, func() bool {
		_, err := os.Stat(sock)
		return err == nil
	}, time.Second, 5*time.Millisecond)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err = New(service.NewClient(sock), "test").Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { session.Close() })
	return session
}

func call[T any](t *testing.T, session *mcp.ClientSession, tool string, args any) T {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err)
	require.False(t, res.IsError, "%s failed: %v", tool, text(res))
	var out T
	require.NoError(t, json.Unmarshal([]byte(text(res)), &out))
	return out
}

func callError(t *testing.T, session *mcp.ClientSession, tool string, args any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err)
	require.True(t, res.IsError, "%s should fail", tool)
	return text(res)
}

func text(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestTools(t *testing.T) {
	session := connect(t)

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	require.Equal(t, []string{"copy_entry", "delete_entries", "get_entry", "list_entries", "set_clipboard"}, names)

	first := call[service.APIEntry](t, session, "set_clipboard", map[string]any{"text": "go test ./..."})
	second := call[service.APIEntry](t, session, "set_clipboard", map[string]any{"text": "kubectl get pods"})
	require.NotEqual(t, first.ID, second.ID)

	listed := call[service.ListEntriesResponse](t, session, "list_entries", map[string]any{"query": "GO TEST"})
	require.Equal(t, 1, listed.Total)
	require.Equal(t, first.ID, listed.Entries[0].ID)

	got := call[service.APIEntry](t, session, "get_entry", map[string]any{"id": first.ID})
	require.Equal(t, "go test ./...", got.Content)

	copied := call[service.APIEntry](t, session, "copy_entry", map[string]any{"id": first.ID})
	require.True(t, copied.Current)
	require.Equal(t, "important", copied.Kind)

	deleted := call[service.DeleteEntriesResponse](t, session, "delete_entries", map[string]any{"ids": []uint64{first.ID, 999}})
	require.Equal(t, []uint64{first.ID}, deleted.Deleted)
	require.Equal(t, []uint64{999}, deleted.NotFound)

	require.Contains(t, callError(t, session, "get_entry", map[string]any{"id": first.ID}), "no entry with id")
	require.Contains(t, callError(t, session, "list_entries", map[string]any{"regex": "("}), "invalid regex")
}

func TestTools_serverNotRunning(t *testing.T) {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err := New(service.NewClient(filepath.Join(t.TempDir(), "missing.sock")), "test").Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	require.Contains(t, callError(t, session, "list_entries", map[string]any{}), "is it running?")
}
