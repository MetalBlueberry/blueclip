package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"time"
)

type Selection struct {
	xclip.Selection
	// ID identifies the entry across restarts. Identical content keeps its ID.
	ID uint64
	// Time is when the entry was last selected or copied. It is zero for
	// entries saved before times were recorded.
	Time time.Time
}

// Line appends a null terminator to the selection
func (s *Selection) Line() []byte {
	return append(s.Clean(), '\000')
}

// Clean returns a byte valid for line comparison
func (s *Selection) Clean() []byte {
	return s.Content
}

// isImage reports whether the selection holds image data. Image support was
// dropped, but histories saved by older versions may still contain them.
func (s *Selection) isImage() bool {
	return strings.HasPrefix(string(s.Target), "image/")
}

func (s *Selection) String() string {
	return string(s.Content)
}

// Compares the actual content
func (s *Selection) Equal(other Selection) bool {
	return bytes.Equal(s.Content, other.Content)
}

type Set struct {
	Ephemeral []Selection
	Important []Selection
	Last      *Selection
	// NextID is the last ID handed out.
	NextID uint64

	Options Options

	lock sync.Mutex
	now  func() time.Time
}

type SelectionRetentionType string

const (
	SelectionRetentionTypeAll       SelectionRetentionType = "all"
	SelectionRetentionTypeEphemeral SelectionRetentionType = "ephemeral"
	SelectionRetentionTypeImportant SelectionRetentionType = "important"
)

func (s *Set) ClearAll(typ SelectionRetentionType) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeEphemeral {
		log.Printf("Clearing all ephemeral selections")
		s.Ephemeral = []Selection{}
	}
	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeImportant {
		log.Printf("Clearing all important selections")
		s.Important = []Selection{}
	}
}

func (s *Set) Clear(pattern []byte, typ SelectionRetentionType) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	found := false

	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeEphemeral {
		filtered := []Selection{}
		for _, sel := range s.Ephemeral {
			if !bytes.Equal(sel.Clean(), pattern) {
				filtered = append(filtered, sel)
			} else {
				log.Printf("clearing ephemeral selection: %d bytes", len(sel.Content))
				found = true
			}
		}
		s.Ephemeral = filtered
	}

	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeImportant {
		filtered := []Selection{}
		for _, sel := range s.Important {
			if !bytes.Equal(sel.Clean(), pattern) {
				filtered = append(filtered, sel)
			} else {
				log.Printf("clearing important selection: %d bytes", len(sel.Content))
				found = true
			}
		}
		s.Important = filtered
	}

	return found
}

type Options struct {
	MaxEphemeralElements int
	MaxImportantElements int
}

func NewSelections() *Set {
	return &Set{
		// Selections at the end of the list are the most recent
		Ephemeral: []Selection{},
		Important: []Selection{},
		Options: Options{
			MaxEphemeralElements: 200,
			MaxImportantElements: 100,
		},
	}
}

// Add records a new selection and returns it as stored, with its ID and time.
func (s *Set) Add(selection Selection) Selection {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Printf("Adding selection: %d bytes", len(selection.Content))

	if s.Last != nil {
		if selection.Equal(*s.Last) {
			log.Printf("Selected content is already in the last selection")
			return *s.Last
		}
	}

	selection.Time = s.clock()
	selection.ID = s.idOf(selection)
	if selection.ID == 0 {
		selection.ID = s.newID()
	}

	log.Printf("Setting last selection")
	s.Last = &selection

	isImportant := false
	{
		filtered := []Selection{}
		for _, sel := range s.Important {
			if !selection.Equal(sel) {
				filtered = append(filtered, sel)
			} else {
				isImportant = true
				log.Printf("Selected content is already in the important list")
			}
		}
		if isImportant {
			s.Important = append(filtered, selection)
		} else {
			s.Important = filtered
		}
	}

	{
		filtered := []Selection{}

		for _, sel := range s.Ephemeral {
			if !bytes.Contains(selection.Content, sel.Content) {
				filtered = append(filtered, sel)
			} else {
				log.Printf("Dropping existing selection as it's contained in new selection")
			}
		}

		if isImportant {
			s.Ephemeral = filtered
		} else {
			s.Ephemeral = append(filtered, selection)
		}
	}

	s.truncate()

	log.Printf("Ephemeral length: %d", len(s.Ephemeral))
	log.Printf("Important length: %d", len(s.Important))
	return selection
}

func (s *Set) truncate() {
	if len(s.Ephemeral) > s.Options.MaxEphemeralElements {
		log.Printf("Truncating ephemeral list to %d elements", s.Options.MaxEphemeralElements)
		s.Ephemeral = s.Ephemeral[len(s.Ephemeral)-s.Options.MaxEphemeralElements:]
	}
	if len(s.Important) > s.Options.MaxImportantElements {
		log.Printf("Truncating important list to %d elements", s.Options.MaxImportantElements)
		s.Important = s.Important[len(s.Important)-s.Options.MaxImportantElements:]
	}
}

func (s *Set) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Set) newID() uint64 {
	s.NextID++
	return s.NextID
}

// idOf returns the ID of an existing entry with the same content, or 0.
func (s *Set) idOf(selection Selection) uint64 {
	for _, list := range [][]Selection{s.Important, s.Ephemeral} {
		for _, sel := range list {
			if sel.ID != 0 && sel.Equal(selection) {
				return sel.ID
			}
		}
	}
	return 0
}

// assignMissingIDs gives IDs to entries saved before IDs existed. Older
// entries get lower IDs, and the last selection reuses the ID of the entry
// with the same content.
func (s *Set) assignMissingIDs() {
	for _, list := range [][]Selection{s.Ephemeral, s.Important} {
		for _, sel := range list {
			s.NextID = max(s.NextID, sel.ID)
		}
	}
	if s.Last != nil {
		s.NextID = max(s.NextID, s.Last.ID)
	}

	for _, list := range [][]Selection{s.Ephemeral, s.Important} {
		for i := range list {
			if list[i].ID == 0 {
				list[i].ID = s.newID()
			}
		}
	}
	if s.Last != nil && s.Last.ID == 0 {
		s.Last.ID = s.idOf(*s.Last)
		if s.Last.ID == 0 {
			s.Last.ID = s.newID()
		}
	}
}

// Encode writes the set while holding the lock, so it is safe to call
// while other goroutines modify the set.
func (s *Set) Encode(enc *gob.Encoder) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return enc.Encode(s)
}

func (s *Set) Decode(dec *gob.Decoder) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if err := dec.Decode(s); err != nil {
		return err
	}

	isImage := func(sel Selection) bool { return sel.isImage() }
	s.Ephemeral = slices.DeleteFunc(s.Ephemeral, isImage)
	s.Important = slices.DeleteFunc(s.Important, isImage)
	if s.Last != nil && s.Last.isImage() {
		s.Last = nil
	}
	s.assignMissingIDs()
	return nil
}

func (s *Set) List(out io.Writer) {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Printf("Listing %d important and %d ephemeral selections", len(s.Important), len(s.Ephemeral))

	// Get max length to know how many iterations we need
	maxLen := len(s.Important)
	if len(s.Ephemeral) > maxLen {
		maxLen = len(s.Ephemeral)
	}

	// Last selection is always written first
	if s.Last != nil {
		out.Write(s.Last.Line())
	}

	// Iterate from the end of both slices
	for i := 0; i < maxLen; i++ {
		// Write Important second if available
		importantIdx := len(s.Important) - 1 - i
		if importantIdx >= 0 {
			if s.Last == nil || !s.Important[importantIdx].Equal(*s.Last) {
				_, err := out.Write(s.Important[importantIdx].Line())
				if err != nil {
					log.Printf("Failed to write important selection: %v", err)
				}
			}
		}

		// Write Ephemeral first if available
		ephemeralIdx := len(s.Ephemeral) - 1 - i
		if ephemeralIdx >= 0 {
			if s.Last == nil || !s.Ephemeral[ephemeralIdx].Equal(*s.Last) {
				_, err := out.Write(s.Ephemeral[ephemeralIdx].Line())
				if err != nil {
					log.Printf("Failed to write ephemeral selection: %v", err)
				}
			}
		}
	}
}

// Copy promotes the entry matching a line from List to the important list
// and makes it the last selection.
func (s *Set) Copy(line []byte) (Selection, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Handle empty line
	if len(line) == 0 {
		return Selection{}, false
	}

	// Remove null terminator if present
	cleanLine := line
	if line[len(line)-1] == '\000' || line[len(line)-1] == '\n' {
		cleanLine = line[:len(line)-1]
	}

	return s.promote(func(sel Selection) bool {
		return bytes.Equal(cleanLine, sel.Clean())
	})
}

// CopyByID is Copy for an entry ID.
func (s *Set) CopyByID(id uint64) (Selection, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.promote(func(sel Selection) bool {
		return sel.ID == id
	})
}

func (s *Set) promote(match func(Selection) bool) (Selection, bool) {
	for i, sel := range s.Important {
		if match(sel) {
			sel.Time = s.clock()
			s.Important = append(slices.Delete(s.Important, i, i+1), sel)
			s.Last = &sel
			return sel, true
		}
	}

	var found *Selection
	filtered := []Selection{}
	for _, sel := range s.Ephemeral {
		if match(sel) {
			log.Printf("Moving selection to important list")
			sel.Time = s.clock()
			s.Important = append(s.Important, sel)
			found = &sel
		} else {
			filtered = append(filtered, sel)
		}
	}
	s.Ephemeral = filtered

	if found == nil {
		return Selection{}, false
	}
	s.Last = found
	s.truncate()
	return *found, true
}

func (s *Set) FindMatch(line []byte) (Selection, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Handle empty line
	if len(line) == 0 {
		return Selection{}, false
	}

	// Remove null terminator if present
	cleanLine := line
	if line[len(line)-1] == '\000' || line[len(line)-1] == '\n' {
		cleanLine = line[:len(line)-1]
	}

	for _, selection := range s.Important {
		if bytes.Equal(cleanLine, selection.Clean()) {
			return selection, true
		}
	}

	for _, selection := range s.Ephemeral {
		if bytes.Equal(cleanLine, selection.Clean()) {
			return selection, true
		}
	}
	return Selection{}, false
}
