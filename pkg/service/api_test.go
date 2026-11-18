package service

import (
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type copied struct {
	args    string
	content string
}

// fakeXclip records copies instead of touching the real clipboard.
type fakeXclip struct {
	mu     sync.Mutex
	copies []copied
}

func useFakeXclip(t *testing.T) *fakeXclip {
	t.Helper()
	f := &fakeXclip{}
	previous := xclip.Cli.RunFn
	xclip.Cli.RunFn = f.run
	t.Cleanup(func() { xclip.Cli.RunFn = previous })
	return f
}

func (f *fakeXclip) run(cmd *exec.Cmd) error {
	content := ""
	if cmd.Stdin != nil {
		b, err := io.ReadAll(cmd.Stdin)
		if err != nil {
			return err
		}
		content = string(b)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copies = append(f.copies, copied{args: strings.Join(cmd.Args[1:], " "), content: content})
	return nil
}

func (f *fakeXclip) Copies() []copied {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]copied(nil), f.copies...)
}

// startAPI serves the service on a temporary socket and returns a client for it.
func startAPI(t *testing.T, contents ...string) (*Service, *Client) {
	t.Helper()
	s := newTestService(t, contents...)
	sock := filepath.Join(t.TempDir(), "blueclip.sock")
	server, err := NewServer(s.Handler(), sock)
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

	return s, NewClient(sock)
}

func addFrom(s *Service, content string, source xclip.ClipboardSelection) selections.Selection {
	return s.selections.Add(selections.Selection{Selection: xclip.Selection{
		Content: []byte(content),
		Target:  xclip.ValidTargetUTF8_STRING,
		Source:  source,
	}})
}

func TestAPI_SetClipboard(t *testing.T) {
	fake := useFakeXclip(t)
	s, client := startAPI(t)
	ctx := context.Background()

	entry, err := client.SetClipboard(ctx, "hello\nworld", nil)
	require.NoError(t, err)

	require.Equal(t, []copied{{args: "-i -selection clipboard -target UTF8_STRING -silent", content: "hello\nworld"}}, fake.Copies())
	require.NotZero(t, entry.ID)
	require.Equal(t, "hello\nworld", entry.Content)
	require.Equal(t, "clipboard", entry.Source)
	require.Equal(t, 2, entry.Lines)
	require.True(t, entry.Current)
	require.NotEmpty(t, entry.Time)

	onDisk := loadFromDisk(t, s)
	require.Equal(t, "hello\nworld", string(onDisk.Last.Content))

	t.Run("empty text is rejected", func(t *testing.T) {
		_, err := client.SetClipboard(ctx, "", nil)
		require.ErrorContains(t, err, "text is empty")
	})

	t.Run("invalid selection is rejected before copying", func(t *testing.T) {
		before := len(fake.Copies())
		_, err := client.SetClipboard(ctx, "x", []string{"bogus"})
		require.ErrorContains(t, err, "invalid selection")
		require.Len(t, fake.Copies(), before)
	})
}

func TestAPI_ListEntries(t *testing.T) {
	useFakeXclip(t)
	s, client := startAPI(t)
	ctx := context.Background()
	addFrom(s, "git commit -m fix", xclip.ClipboardSelectionClipboard)
	addFrom(s, "highlighted text", xclip.ClipboardSelectionPrimary)
	addFrom(s, strings.Repeat("é", 400), xclip.ClipboardSelectionClipboard)

	t.Run("defaults truncate long content", func(t *testing.T) {
		got, err := client.ListEntries(ctx, ListQuery{})
		require.NoError(t, err)
		require.Equal(t, 3, got.Total)
		require.Len(t, got.Entries, 3)
		long := got.Entries[0]
		require.True(t, long.Truncated)
		require.Equal(t, DefaultListMaxChars, len([]rune(long.Content)))
		require.Equal(t, 800, long.Bytes)
	})

	t.Run("filters", func(t *testing.T) {
		got, err := client.ListEntries(ctx, ListQuery{Query: "GIT fix"})
		require.NoError(t, err)
		require.Len(t, got.Entries, 1)
		require.Equal(t, "git commit -m fix", got.Entries[0].Content)

		got, err = client.ListEntries(ctx, ListQuery{Source: "primary"})
		require.NoError(t, err)
		require.Len(t, got.Entries, 1)
		require.Equal(t, "highlighted text", got.Entries[0].Content)

		got, err = client.ListEntries(ctx, ListQuery{Regex: `^git `, Since: "1h"})
		require.NoError(t, err)
		require.Len(t, got.Entries, 1)
	})

	t.Run("limit and offset", func(t *testing.T) {
		got, err := client.ListEntries(ctx, ListQuery{Limit: 1, Offset: 1, MaxChars: -1})
		require.NoError(t, err)
		require.Equal(t, 3, got.Total)
		require.Len(t, got.Entries, 1)
		require.Equal(t, "highlighted text", got.Entries[0].Content)
	})

	t.Run("invalid filters are explained", func(t *testing.T) {
		_, err := client.ListEntries(ctx, ListQuery{Regex: "("})
		require.ErrorContains(t, err, "invalid regex")
		_, err = client.ListEntries(ctx, ListQuery{Since: "yesterday-ish"})
		require.ErrorContains(t, err, "invalid since")
		_, err = client.ListEntries(ctx, ListQuery{Kind: "secret"})
		require.ErrorContains(t, err, "invalid kind")
	})
}

func TestAPI_GetEntry(t *testing.T) {
	useFakeXclip(t)
	s, client := startAPI(t)
	ctx := context.Background()
	long := addFrom(s, strings.Repeat("x", 1000), xclip.ClipboardSelectionClipboard)

	got, err := client.GetEntry(ctx, long.ID)
	require.NoError(t, err)
	require.Len(t, got.Content, 1000)
	require.False(t, got.Truncated)

	_, err = client.GetEntry(ctx, 999)
	require.ErrorContains(t, err, "no entry with id 999")
}

func TestAPI_CopyEntry(t *testing.T) {
	fake := useFakeXclip(t)
	s, client := startAPI(t)
	ctx := context.Background()
	a := addFrom(s, "Selection A", xclip.ClipboardSelectionClipboard)
	addFrom(s, "Selection B", xclip.ClipboardSelectionClipboard)

	got, err := client.CopyEntry(ctx, a.ID, []string{"clipboard", "primary"})
	require.NoError(t, err)

	require.Equal(t, a.ID, got.ID)
	require.Equal(t, "important", got.Kind)
	require.True(t, got.Current)
	require.Equal(t, []copied{
		{args: "-i -selection clipboard -target UTF8_STRING -silent", content: "Selection A"},
		{args: "-i -selection primary -target UTF8_STRING -silent", content: "Selection A"},
	}, fake.Copies())

	_, err = client.CopyEntry(ctx, 999, nil)
	require.ErrorContains(t, err, "no entry with id 999")
}

func TestAPI_DeleteEntries(t *testing.T) {
	useFakeXclip(t)
	s, client := startAPI(t)
	ctx := context.Background()
	a := addFrom(s, "Selection A", xclip.ClipboardSelectionClipboard)
	b := addFrom(s, "Selection B", xclip.ClipboardSelectionClipboard)

	got, err := client.DeleteEntries(ctx, []uint64{a.ID, 999})
	require.NoError(t, err)
	require.Equal(t, []uint64{a.ID}, got.Deleted)
	require.Equal(t, []uint64{999}, got.NotFound)

	onDisk := loadFromDisk(t, s)
	require.Len(t, onDisk.Ephemeral, 1)
	require.Equal(t, b.ID, onDisk.Ephemeral[0].ID)

	_, err = client.DeleteEntries(ctx, nil)
	require.ErrorContains(t, err, "ids is empty")
}

func TestParseTime(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2026, 11, 18, 15, 30, 0, 0, loc)

	tests := map[string]time.Time{
		"":                     {},
		"2h":                   now.Add(-2 * time.Hour),
		"90m":                  now.Add(-90 * time.Minute),
		"3d":                   now.Add(-72 * time.Hour),
		"2026-11-18":           time.Date(2026, 11, 18, 0, 0, 0, 0, loc),
		"2026-11-18T09:15":     time.Date(2026, 11, 18, 9, 15, 0, 0, loc),
		"2026-11-18T09:15:00Z": time.Date(2026, 11, 18, 9, 15, 0, 0, time.UTC),
	}
	for raw, want := range tests {
		got, err := parseTime(raw, now)
		require.NoError(t, err, raw)
		require.True(t, want.Equal(got), "%s: want %v got %v", raw, want, got)
	}

	_, err := parseTime("last tuesday", now)
	require.Error(t, err)
}
