package service

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultSocketPath(t *testing.T) {
	t.Run("uses XDG_RUNTIME_DIR when set", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1234")
		require.Equal(t, "/run/user/1234/blueclip/blueclip.sock", DefaultSocketPath())
	})

	t.Run("falls back to a per-user directory in the temp dir", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "")
		want := filepath.Join(os.TempDir(), fmt.Sprintf("blueclip-%d", os.Getuid()), "blueclip.sock")
		require.Equal(t, want, DefaultSocketPath())
	})
}

func TestEnsurePrivateDir(t *testing.T) {
	t.Run("creates a missing directory with 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "blueclip")
		require.NoError(t, ensurePrivateDir(dir))

		info, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})

	t.Run("tightens permissions of an existing directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "blueclip")
		require.NoError(t, os.Mkdir(dir, 0777))
		require.NoError(t, os.Chmod(dir, 0777))

		require.NoError(t, ensurePrivateDir(dir))

		info, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})

	t.Run("rejects a symlink", func(t *testing.T) {
		base := t.TempDir()
		target := filepath.Join(base, "target")
		require.NoError(t, os.Mkdir(target, 0700))
		link := filepath.Join(base, "blueclip")
		require.NoError(t, os.Symlink(target, link))

		require.Error(t, ensurePrivateDir(link))
	})
}

func TestNewServer(t *testing.T) {
	t.Run("listens on the given socket with owner-only permissions", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "s", "blueclip.sock")
		server, err := NewServer(http.NewServeMux(), sock)
		require.NoError(t, err)

		errCh := make(chan error, 1)
		go func() { errCh <- server.ListenAndServe() }()
		t.Cleanup(func() {
			server.Close()
			<-errCh
		})

		require.Eventually(t, func() bool {
			info, err := os.Stat(sock)
			return err == nil && info.Mode().Perm() == 0600
		}, time.Second, 10*time.Millisecond)
	})
}
