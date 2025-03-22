package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"io"
	"log"
)

type Selection struct {
	xclip.Selection
}

func (s *Selection) Line() []byte {
	singleLine := bytes.ReplaceAll(s.Content, []byte("\n"), []byte(" "))
	return append(bytes.TrimSpace(singleLine), '\n')
}

func (s *Selection) String() string {
	return string(s.Content)
}

type Set struct {
	Ephemeral []Selection
	Important []Selection

	Options Options
}

type Options struct {
	MaxElements int
}

func NewSelections() *Set {
	return &Set{
		Ephemeral: []Selection{},
		Important: []Selection{},
		Options: Options{
			MaxElements: 200,
		},
	}
}

func (s *Set) Add(selection Selection) {
	log.Printf("Adding selection: %d bytes", len(selection.Content))

	filtered := []Selection{}

	for _, sel := range s.Ephemeral {
		if !bytes.Contains(selection.Content, sel.Content) {
			filtered = append(filtered, sel)
		} else {
			log.Printf("Dropping existing selection as it's contained in new selection")
		}
	}

	s.Ephemeral = filtered
	s.Ephemeral = append(s.Ephemeral, selection)

	if len(s.Ephemeral) > s.Options.MaxElements {
		s.Ephemeral = s.Ephemeral[len(s.Ephemeral)-s.Options.MaxElements:]
	}
}

func (s *Set) List(out io.Writer) {
	log.Printf("Listing %d important selections", len(s.Important))
	for _, selection := range s.Important {
		_, err := out.Write(selection.Line())
		if err != nil {
			log.Printf("Failed to write selection: %v", err)
		}
	}
	log.Printf("Listing %d ephemeral selections", len(s.Ephemeral))
	for _, selection := range s.Ephemeral {
		_, err := out.Write(selection.Line())
		if err != nil {
			log.Printf("Failed to write selection: %v", err)
		}
	}
}

func (s *Set) FindMatch(line []byte) (Selection, bool) {
	for _, selection := range s.Important {
		if bytes.Equal(line, selection.Line()) {
			return selection, true
		}
	}

	for _, selection := range s.Ephemeral {
		if bytes.Equal(line, selection.Line()) {
			return selection, true
		}
	}
	return Selection{}, false
}
