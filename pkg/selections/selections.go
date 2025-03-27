package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image/png"
	"io"
	"log"
	"sync"
)

type Selection struct {
	xclip.Selection
}

func (s *Selection) Line() []byte {
	if s.Target == xclip.ValidTargetImagePng {
		img, err := png.Decode(bytes.NewReader(s.Content))
		if err != nil {
			return []byte("Failed to decode image")
		}

		hash := md5.Sum(s.Content)
		hashStr := hex.EncodeToString(hash[:])
		return fmt.Appendf(nil, "PNG Image: %d x %d %s\n", img.Bounds().Max.X, img.Bounds().Max.Y, hashStr)
	}
	// singleLine := bytes.ReplaceAll(s.Content, []byte("\n"), []byte(" "))
	// return append(bytes.TrimSpace(singleLine), '\n')
	return append(bytes.TrimSpace(s.Content), '\000')
}

func (s *Selection) String() string {
	return string(s.Content)
}

func (s *Selection) Equal(other Selection) bool {
	return bytes.Equal(s.Content, other.Content)
}

type Set struct {
	Ephemeral []Selection
	Important []Selection
	Last      *Selection

	Options Options

	lock sync.Mutex
}

type SelectionRetentionType string

const (
	SelectionRetentionTypeAll       SelectionRetentionType = "all"
	SelectionRetentionTypeEphemeral SelectionRetentionType = "ephemeral"
	SelectionRetentionTypeImportant SelectionRetentionType = "important"
)

func (s *Set) Clear(pattern string, typ SelectionRetentionType) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if pattern == "" {
		if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeEphemeral {
			s.Ephemeral = []Selection{}
		}
		if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeImportant {
			s.Important = []Selection{}
		}
		return
	}

	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeEphemeral {
		filtered := []Selection{}
		for _, sel := range s.Ephemeral {
			if !bytes.Equal(sel.Line(), []byte(pattern)) {
				filtered = append(filtered, sel)
			} else {
				log.Printf("clearing ephemeral selection: %d bytes", len(sel.Content))
			}
		}
		s.Ephemeral = filtered
	}

	if typ == SelectionRetentionTypeAll || typ == SelectionRetentionTypeImportant {
		filtered := []Selection{}
		for _, sel := range s.Important {
			if !bytes.Equal(sel.Line(), []byte(pattern)) {
				filtered = append(filtered, sel)
			} else {
				log.Printf("clearing important selection: %d bytes", len(sel.Content))
			}
		}
		s.Important = filtered
	}
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

func (s *Set) Add(selection Selection) {
	s.lock.Lock()
	defer s.lock.Unlock()

	log.Printf("Adding selection: %d bytes", len(selection.Content))

	if s.Last != nil {
		if selection.Equal(*s.Last) {
			log.Printf("Selected content is already in the last selection")
			return
		}
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

	if len(s.Ephemeral) > s.Options.MaxEphemeralElements {
		log.Printf("Truncating ephemeral list to %d elements", s.Options.MaxEphemeralElements)
		s.Ephemeral = s.Ephemeral[len(s.Ephemeral)-s.Options.MaxEphemeralElements:]
	}
	if len(s.Important) > s.Options.MaxImportantElements {
		log.Printf("Truncating important list to %d elements", s.Options.MaxImportantElements)
		s.Important = s.Important[len(s.Important)-s.Options.MaxImportantElements:]
	}

	log.Printf("Ephemeral length: %d", len(s.Ephemeral))
	log.Printf("Important length: %d", len(s.Important))
}

func (s *Set) List(out io.Writer) {
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
			if s.Last != nil && !s.Important[importantIdx].Equal(*s.Last) {
				_, err := out.Write(s.Important[importantIdx].Line())
				if err != nil {
					log.Printf("Failed to write important selection: %v", err)
				}
			}
		}

		// Write Ephemeral first if available
		ephemeralIdx := len(s.Ephemeral) - 1 - i
		if ephemeralIdx >= 0 {
			if s.Last != nil && !s.Ephemeral[ephemeralIdx].Equal(*s.Last) {
				_, err := out.Write(s.Ephemeral[ephemeralIdx].Line())
				if err != nil {
					log.Printf("Failed to write ephemeral selection: %v", err)
				}
			}
		}
	}
}

func (s *Set) Copy(line []byte) (Selection, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Last character is a null terminator or a new line
	cleanLine := line[:len(line)-1]

	for i, selection := range s.Important {
		if bytes.Equal(cleanLine, selection.Content) {
			s.Last = &selection
			s.Important = append(s.Important[:i], s.Important[i+1:]...)
			s.Important = append(s.Important, selection)
			return selection, true
		}
	}

	sel := Selection{}
	found := false
	filtered := []Selection{}
	for _, selection := range s.Ephemeral {
		if bytes.Equal(cleanLine, selection.Content) {
			log.Printf("Moving selection to important list")
			sel = selection
			found = true
			s.Important = append(s.Important, selection)
		} else {
			filtered = append(filtered, selection)
		}
	}
	s.Ephemeral = filtered

	if found {
		s.Last = &sel
		return sel, true
	}

	return Selection{}, false
}

func (s *Set) FindMatch(line []byte) (Selection, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Last character is a null terminator or a new line
	cleanLine := line[:len(line)-1]

	for _, selection := range s.Important {
		if bytes.Equal(cleanLine, selection.Content) {
			return selection, true
		}
	}

	for _, selection := range s.Ephemeral {
		if bytes.Equal(cleanLine, selection.Content) {
			return selection, true
		}
	}
	return Selection{}, false
}
