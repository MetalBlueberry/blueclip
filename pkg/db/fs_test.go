package db

import (
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newSet(contents ...string) *selections.Set {
	s := selections.NewSelections()
	for _, c := range contents {
		s.Add(selections.Selection{
			Selection: xclip.NewSelection([]byte(c), xclip.ValidTargetUTF8_STRING),
		})
	}
	return s
}

func TestFileDB_Save(t *testing.T) {
	t.Run("round trips through load", func(t *testing.T) {
		db := &FileDB{Path: filepath.Join(t.TempDir(), "nested", "history.bin")}
		require.NoError(t, db.Save(newSet("Selection A", "Selection B")))

		loaded := selections.NewSelections()
		require.NoError(t, db.Load(loaded))
		require.Len(t, loaded.Ephemeral, 2)
		require.Equal(t, "Selection B", string(loaded.Last.Content))
	})

	t.Run("file is only readable by the owner", func(t *testing.T) {
		db := &FileDB{Path: filepath.Join(t.TempDir(), "history.bin")}
		require.NoError(t, os.WriteFile(db.Path, []byte("old"), 0664))

		require.NoError(t, db.Save(newSet("Selection A")))

		info, err := os.Stat(db.Path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("leaves no temporary files behind", func(t *testing.T) {
		dir := t.TempDir()
		db := &FileDB{Path: filepath.Join(dir, "history.bin")}
		require.NoError(t, db.Save(newSet("Selection A")))
		require.NoError(t, db.Save(newSet("Selection B")))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "history.bin", entries[0].Name())
	})

	t.Run("failed save keeps the previous file intact", func(t *testing.T) {
		dir := t.TempDir()
		db := &FileDB{Path: filepath.Join(dir, "history.bin")}
		require.NoError(t, db.Save(newSet("Selection A")))

		require.NoError(t, os.Chmod(dir, 0500))
		t.Cleanup(func() { os.Chmod(dir, 0700) })
		require.Error(t, db.Save(newSet("Selection B")))

		loaded := selections.NewSelections()
		require.NoError(t, db.Load(loaded))
		require.Equal(t, "Selection A", string(loaded.Last.Content))
	})
}
