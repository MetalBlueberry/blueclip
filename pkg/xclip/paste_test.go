package xclip

import (
	"bytes"
	"context"
	"testing"
)

func TestPaste(t *testing.T) {
	t.Run("test paste", func(t *testing.T) {
		execer := NewMockExecer(t)
		execer.AddCall("xclip -o -silent", []byte("test"))

		xclip := XClip{
			RunFn: execer.Run,
		}

		buf := &bytes.Buffer{}
		err := xclip.Paste(context.Background(), buf)
		if err != nil {
			t.Errorf("Paste() error = %v", err)
		}

		if buf.String() != "test" {
			t.Errorf("Paste() = %v, want %v", buf.String(), "test")
		}
	})

	t.Run("test paste with selection", func(t *testing.T) {
		execer := NewMockExecer(t)
		execer.AddCall("xclip -o -selection primary -silent", []byte("test"))

		xclip := XClip{
			RunFn: execer.Run,
		}

		buf := &bytes.Buffer{}
		err := xclip.Paste(context.Background(), buf, PasteOptionWithSelection(ClipboardSelectionPrimary))
		if err != nil {
			t.Errorf("Paste() error = %v", err)
		}

		if buf.String() != "test" {
			t.Errorf("Paste() = %v, want %v", buf.String(), "test")
		}
	})
}
