package service

import (
	"blueclip/pkg/db"
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, contents ...string) *Service {
	t.Helper()
	s := NewService(&db.FileDB{Path: filepath.Join(t.TempDir(), "history.bin")}, "")
	for _, c := range contents {
		s.selections.Add(selections.Selection{
			Selection: xclip.NewSelection([]byte(c), xclip.ValidTargetUTF8_STRING),
		})
	}
	require.NoError(t, s.persist())
	return s
}

func loadFromDisk(t *testing.T, s *Service) *selections.Set {
	t.Helper()
	loaded := selections.NewSelections()
	require.NoError(t, s.db.Load(loaded))
	return loaded
}

func TestHandleClear(t *testing.T) {
	t.Run("clear all with the client options removes every selection", func(t *testing.T) {
		s := newTestService(t, "Selection A", "Selection B")
		s.selections.Copy([]byte("Selection B"))
		require.Len(t, s.selections.Ephemeral, 1)
		require.Len(t, s.selections.Important, 1)

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", nil)
		ClearWithType("all")(req)
		ClearWithAll(true)(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, s.selections.Ephemeral)
		require.Empty(t, s.selections.Important)

		onDisk := loadFromDisk(t, s)
		require.Empty(t, onDisk.Ephemeral)
		require.Empty(t, onDisk.Important)
	})

	t.Run("clear without patterns or all is rejected", func(t *testing.T) {
		s := newTestService(t, "Selection A")

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", nil)
		ClearWithType("all")(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Len(t, s.selections.Ephemeral, 1)
	})

	t.Run("empty patterns between valid ones are skipped", func(t *testing.T) {
		s := newTestService(t, "Selection A", "Selection B", "Selection C")

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", strings.NewReader("\000Selection A\000\000Selection B\000"))
		ClearWithType("all")(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Len(t, s.selections.Ephemeral, 1)
		require.Equal(t, "Selection C", string(s.selections.Ephemeral[0].Content))
	})

	t.Run("clear by pattern removes only the matching selection", func(t *testing.T) {
		s := newTestService(t, "Selection A", "Selection B")

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", strings.NewReader("Selection A\000"))
		ClearWithType("all")(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, s.selections.Ephemeral, 1)
		require.Equal(t, "Selection B", string(s.selections.Ephemeral[0].Content))

		onDisk := loadFromDisk(t, s)
		require.Len(t, onDisk.Ephemeral, 1)
		require.Equal(t, "Selection B", string(onDisk.Ephemeral[0].Content))
	})

	t.Run("patterns cleared before a missing one are persisted", func(t *testing.T) {
		s := newTestService(t, "Selection A", "Selection B")

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", strings.NewReader("Selection A\000Missing\000"))
		ClearWithType("all")(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		onDisk := loadFromDisk(t, s)
		require.Len(t, onDisk.Ephemeral, 1)
		require.Equal(t, "Selection B", string(onDisk.Ephemeral[0].Content))
	})

	t.Run("invalid type is rejected", func(t *testing.T) {
		s := newTestService(t, "Selection A")

		req := httptest.NewRequest(http.MethodPost, "http://blueclip/clear", nil)
		ClearWithType("bogus")(req)
		ClearWithAll(true)(req)

		rec := httptest.NewRecorder()
		s.HandleClear(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Len(t, s.selections.Ephemeral, 1)
	})
}
