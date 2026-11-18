package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"encoding/gob"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeClock returns a clock that advances one minute on every call, starting
// at a fixed time, so ordering by time is deterministic.
func fakeClock() func() time.Time {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		now = now.Add(time.Minute)
		return now
	}
}

func newClockedSet() *Set {
	s := NewSelections()
	s.now = fakeClock()
	return s
}

func sel(content string, source xclip.ClipboardSelection) Selection {
	return Selection{Selection: xclip.Selection{
		Content: []byte(content),
		Target:  xclip.ValidTargetUTF8_STRING,
		Source:  source,
	}}
}

func ids(entries []Entry) []uint64 {
	out := []uint64{}
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

func TestSet_Add_assignsIDAndTime(t *testing.T) {
	s := newClockedSet()

	a := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	b := s.Add(sel("Selection B", xclip.ClipboardSelectionPrimary))

	require.Equal(t, uint64(1), a.ID)
	require.Equal(t, uint64(2), b.ID)
	require.True(t, b.Time.After(a.Time))
	require.Equal(t, b.ID, s.Last.ID)
	require.Equal(t, xclip.ClipboardSelectionPrimary, s.Ephemeral[1].Source)
}

func TestSet_Add_reusesIDOfIdenticalContent(t *testing.T) {
	s := newClockedSet()
	a := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	s.Add(sel("Selection B", xclip.ClipboardSelectionClipboard))

	again := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))

	require.Equal(t, a.ID, again.ID)
	require.True(t, again.Time.After(a.Time))
	require.Len(t, s.Ephemeral, 2)
}

func TestSet_Add_sameAsLastReturnsLast(t *testing.T) {
	s := newClockedSet()
	a := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))

	again := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))

	require.Equal(t, a.ID, again.ID)
}

func TestSet_Copy_updatesTimeAndKeepsID(t *testing.T) {
	s := newClockedSet()
	a := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	s.Add(sel("Selection B", xclip.ClipboardSelectionClipboard))

	copied, ok := s.Copy([]byte("Selection A\000"))
	require.True(t, ok)
	require.Equal(t, a.ID, copied.ID)
	require.True(t, copied.Time.After(a.Time))
	require.Equal(t, a.ID, s.Important[0].ID)
}

func TestSet_Decode_assignsIDsToOldEntries(t *testing.T) {
	a := sel("Selection A", "")
	b := sel("Selection B", "")
	c := sel("Selection C", "")
	last := c
	old := &Set{
		Ephemeral: []Selection{a, b},
		Important: []Selection{c},
		Last:      &last,
		Options:   NewSelections().Options,
	}
	buf := bytes.NewBuffer(nil)
	require.NoError(t, old.Encode(gob.NewEncoder(buf)))

	s := NewSelections()
	require.NoError(t, s.Decode(gob.NewDecoder(buf)))

	require.Equal(t, uint64(1), s.Important[0].ID, "important entries are numbered first")
	require.Equal(t, uint64(2), s.Ephemeral[0].ID)
	require.Equal(t, uint64(3), s.Ephemeral[1].ID)
	require.Equal(t, uint64(1), s.Last.ID, "last keeps the ID of the entry with the same content")
	require.Equal(t, []uint64{3, 2, 1}, ids(s.Entries(Filter{})), "recent ephemeral entries list first")
	require.True(t, s.Ephemeral[0].Time.IsZero(), "old entries have no known time")

	s.now = fakeClock()
	d := s.Add(sel("Selection D", xclip.ClipboardSelectionClipboard))
	require.Equal(t, uint64(4), d.ID, "new IDs continue after the migrated ones")
}

func TestSet_Decode_keepsExistingIDs(t *testing.T) {
	s := newClockedSet()
	s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	s.Add(sel("Selection B", xclip.ClipboardSelectionClipboard))
	buf := bytes.NewBuffer(nil)
	require.NoError(t, s.Encode(gob.NewEncoder(buf)))

	loaded := NewSelections()
	require.NoError(t, loaded.Decode(gob.NewDecoder(buf)))

	require.Equal(t, []uint64{1, 2}, []uint64{loaded.Ephemeral[0].ID, loaded.Ephemeral[1].ID})
	loaded.now = fakeClock()
	require.Equal(t, uint64(3), loaded.Add(sel("Selection C", xclip.ClipboardSelectionClipboard)).ID)
}

func TestSet_Entries(t *testing.T) {
	s := newClockedSet()
	s.Add(sel("git commit -m fix", xclip.ClipboardSelectionClipboard))   // 1, 10:01
	s.Add(sel("Hello World", xclip.ClipboardSelectionPrimary))           // 2, 10:02
	s.Add(sel("go test ./...", xclip.ClipboardSelectionClipboard))       // 3, 10:03
	s.Add(sel("SELECT * FROM users", xclip.ClipboardSelectionClipboard)) // 4, 10:04
	s.Copy([]byte("Hello World"))                                        // 2 -> important, 10:05

	t.Run("newest first with kind and current", func(t *testing.T) {
		entries := s.Entries(Filter{})
		require.Equal(t, []uint64{2, 4, 3, 1}, ids(entries))
		require.Equal(t, KindImportant, entries[0].Kind)
		require.True(t, entries[0].Current)
		require.Equal(t, KindEphemeral, entries[1].Kind)
		require.False(t, entries[1].Current)
	})

	t.Run("terms are case insensitive and all must match", func(t *testing.T) {
		require.Equal(t, []uint64{2}, ids(s.Entries(Filter{Terms: []string{"hello", "WORLD"}})))
		require.Empty(t, s.Entries(Filter{Terms: []string{"hello", "git"}}))
	})

	t.Run("regex", func(t *testing.T) {
		require.Equal(t, []uint64{3, 1}, ids(s.Entries(Filter{Regex: regexp.MustCompile(`^(go|git) `)})))
	})

	t.Run("source and kind", func(t *testing.T) {
		require.Equal(t, []uint64{2}, ids(s.Entries(Filter{Source: xclip.ClipboardSelectionPrimary})))
		require.Equal(t, []uint64{4, 3, 1}, ids(s.Entries(Filter{Kind: KindEphemeral})))
	})

	t.Run("time range is inclusive", func(t *testing.T) {
		since := time.Date(2026, 11, 18, 10, 3, 0, 0, time.UTC)
		until := time.Date(2026, 11, 18, 10, 4, 0, 0, time.UTC)
		require.Equal(t, []uint64{4, 3}, ids(s.Entries(Filter{Since: since, Until: until})))
	})
}

func TestSet_Entries_unknownTime(t *testing.T) {
	s := newClockedSet()
	s.Ephemeral = []Selection{{Selection: xclip.NewSelection([]byte("old"), xclip.ValidTargetUTF8_STRING), ID: 7}}
	s.NextID = 7
	s.Add(sel("new", xclip.ClipboardSelectionClipboard))

	require.Equal(t, []uint64{8, 7}, ids(s.Entries(Filter{})), "entries without time sort last")
	require.Equal(t, []uint64{8}, ids(s.Entries(Filter{Since: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)})), "since excludes unknown times")
	require.Equal(t, []uint64{7}, ids(s.Entries(Filter{Until: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)})), "until includes unknown times")
}

func TestSet_Entry(t *testing.T) {
	s := newClockedSet()
	s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))

	e, ok := s.Entry(1)
	require.True(t, ok)
	require.Equal(t, "Selection A", string(e.Content))

	_, ok = s.Entry(99)
	require.False(t, ok)
}

func TestSet_Delete(t *testing.T) {
	s := newClockedSet()
	s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	s.Add(sel("Selection B", xclip.ClipboardSelectionClipboard))
	s.Copy([]byte("Selection A"))
	s.Add(sel("Selection C", xclip.ClipboardSelectionClipboard))

	deleted, notFound := s.Delete([]uint64{1, 3, 42})

	require.Equal(t, []uint64{1, 3}, deleted)
	require.Equal(t, []uint64{42}, notFound)
	require.Equal(t, []uint64{2}, ids(s.Entries(Filter{})))
	require.Nil(t, s.Last, "deleting the current entry removes it from the last selection too")
}

func TestSet_CopyByID(t *testing.T) {
	s := newClockedSet()
	a := s.Add(sel("Selection A", xclip.ClipboardSelectionClipboard))
	s.Add(sel("Selection B", xclip.ClipboardSelectionClipboard))

	copied, ok := s.CopyByID(a.ID)
	require.True(t, ok)
	require.Equal(t, "Selection A", string(copied.Content))
	require.Equal(t, a.ID, s.Last.ID)
	e, _ := s.Entry(a.ID)
	require.Equal(t, KindImportant, e.Kind)

	_, ok = s.CopyByID(99)
	require.False(t, ok)
}
