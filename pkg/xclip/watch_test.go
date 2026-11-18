package xclip

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatch(t *testing.T) {
	t.Run("test watch for UTF8_STRING changes", func(t *testing.T) {
		execer := NewMockExecer(t)
		execer.AddCallN("xclip -o -target TARGETS -selection clipboard -silent", []byte(ValidTargetUTF8_STRING), 3)
		// initial read
		execer.AddCall("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("test"))
		// first poll, unchanged
		execer.AddCall("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("test"))
		// second poll, changed. Monitor read and then content read
		execer.AddCallN("xclip -o -selection clipboard -target UTF8_STRING -silent", []byte("new content"), 2)

		xclip := XClip{
			RunFn: execer.Run,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := xclip.Watch(
			ctx,
			WatchOptionWithClipboardSelection(ClipboardSelectionClipboard),
			WatchOptionWithMonitorTargets([]ValidTarget{ValidTargetUTF8_STRING}),
			WatchOptionWithTargetPriority([]ValidTarget{ValidTargetUTF8_STRING}),
			WatchOptionWithFrequency(time.Millisecond), // Speed up the test
		)
		require.NoError(t, err)

		select {
		case changes := <-ch:
			require.Equal(t, "new content", string(changes.Content))
			require.Equal(t, ValidTargetUTF8_STRING, changes.Target)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for clipboard change")
		}

		cancel()
		for range ch {
		}
	})

	t.Run("invalid frequency returns error", func(t *testing.T) {
		xclip := XClip{RunFn: NewMockExecer(t).Run}
		_, err := xclip.Watch(context.Background(), WatchOptionWithFrequency(0))
		require.Error(t, err)
	})
}
