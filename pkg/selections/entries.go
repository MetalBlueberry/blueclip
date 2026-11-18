package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"regexp"
	"slices"
	"time"
)

type Kind string

const (
	KindEphemeral Kind = "ephemeral"
	KindImportant Kind = "important"
)

// Entry is a selection as seen from outside the set.
type Entry struct {
	Selection
	Kind Kind
	// Current reports whether this is the last selection.
	Current bool
}

// Filter selects entries. Zero fields don't filter.
type Filter struct {
	// Terms must all appear in the content, ignoring case.
	Terms []string
	Regex *regexp.Regexp
	// Since excludes entries without a known time, Until includes them, as
	// they predate the recording of times.
	Since  time.Time
	Until  time.Time
	Source xclip.ClipboardSelection
	Kind   Kind
}

func (f Filter) match(e Entry) bool {
	if f.Kind != "" && e.Kind != f.Kind {
		return false
	}
	if f.Source != "" && e.Source != f.Source {
		return false
	}
	if !f.Since.IsZero() && (e.Time.IsZero() || e.Time.Before(f.Since)) {
		return false
	}
	if !f.Until.IsZero() && e.Time.After(f.Until) {
		return false
	}
	if len(f.Terms) > 0 {
		content := bytes.ToLower(e.Content)
		for _, term := range f.Terms {
			if !bytes.Contains(content, bytes.ToLower([]byte(term))) {
				return false
			}
		}
	}
	if f.Regex != nil && !f.Regex.Match(e.Content) {
		return false
	}
	return true
}

func (s *Set) entries() []Entry {
	out := make([]Entry, 0, len(s.Important)+len(s.Ephemeral))
	for kind, list := range map[Kind][]Selection{KindImportant: s.Important, KindEphemeral: s.Ephemeral} {
		for _, sel := range list {
			out = append(out, Entry{
				Selection: sel,
				Kind:      kind,
				Current:   s.Last != nil && s.Last.ID == sel.ID,
			})
		}
	}
	return out
}

// Entries returns the entries matching the filter, newest first. Entries
// without a known time come last, newest ID first.
func (s *Set) Entries(f Filter) []Entry {
	s.lock.Lock()
	defer s.lock.Unlock()

	out := slices.DeleteFunc(s.entries(), func(e Entry) bool { return !f.match(e) })
	slices.SortFunc(out, func(a, b Entry) int {
		if c := b.Time.Compare(a.Time); c != 0 {
			return c
		}
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		}
		return 0
	})
	return out
}

func (s *Set) Entry(id uint64) (Entry, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, e := range s.entries() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Delete removes entries by ID. Unlike Clear, deleting the last selection
// also forgets it, so the content no longer appears anywhere in the history.
func (s *Set) Delete(ids []uint64) (deleted, notFound []uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	deleted, notFound = []uint64{}, []uint64{}
	for _, id := range ids {
		found := false
		match := func(sel Selection) bool {
			if sel.ID == id {
				found = true
				return true
			}
			return false
		}
		s.Ephemeral = slices.DeleteFunc(s.Ephemeral, match)
		s.Important = slices.DeleteFunc(s.Important, match)
		if s.Last != nil && s.Last.ID == id {
			s.Last = nil
			found = true
		}
		if found {
			deleted = append(deleted, id)
		} else {
			notFound = append(notFound, id)
		}
	}
	return deleted, notFound
}
