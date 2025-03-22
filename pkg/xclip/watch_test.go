package xclip

import (
	"context"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	t.Run("test watch for UTF8_STRING changes", func(t *testing.T) {
		execer := NewMockExecer(t)
		execer.AddCallN("xclip -o -target TARGETS -selection clipboard -silent", []byte(ValidTargetUTF8_STRING), 3)
		execer.AddCall("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("test"))
		execer.AddCall("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("test"))
		execer.AddCall("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("new content"))

		xclip := XClip{
			RunFn: execer.Run,
		}

		ch := xclip.Watch(
			context.Background(),
			WatchOptionWithClipboardSelection(ClipboardSelectionClipboard),
			WatchOptionWithTargets([]ValidTarget{ValidTargetUTF8_STRING}),
			WatchOptionWithFrequency(time.Millisecond), // Speed up the test
		)
		changes := <-ch

		if string(changes.Content) != "new content" {
			t.Errorf("Paste() = %v, want %v", changes.Content, "new content")
		}
	})
}
