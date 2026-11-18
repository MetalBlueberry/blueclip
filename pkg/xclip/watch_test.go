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
			require.Equal(t, ClipboardSelectionClipboard, changes.Source)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for clipboard change")
		}

		cancel()
		for range ch {
		}
	})

	t.Run("skips selections marked as secret by a password manager", func(t *testing.T) {
		const targets = "xclip -o -target TARGETS -selection clipboard -silent"
		const utf8 = "xclip -o -selection clipboard -target UTF8_STRING -silent"
		const hint = "xclip -o -selection clipboard -target x-kde-passwordManagerHint -silent"

		execer := NewMockExecer(t)
		// initial read
		execer.AddCall(targets, []byte("UTF8_STRING"))
		execer.AddCall(utf8, []byte("test"))
		// first poll, a password manager copies a secret
		execer.AddCall(targets, []byte("UTF8_STRING\nx-kde-passwordManagerHint"))
		execer.AddCall(utf8, []byte("hunter2"))
		execer.AddCall(hint, []byte("secret"))
		// second poll, a regular copy
		execer.AddCall(targets, []byte("UTF8_STRING"))
		// monitor and content reads differ so a missing skip shows up as the wrong content
		execer.AddCall(utf8, []byte("monitor read"))
		execer.AddCall(utf8, []byte("new content"))

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
			WatchOptionWithFrequency(time.Millisecond),
			WatchOptionWithSkipSecrets(),
		)
		require.NoError(t, err)

		select {
		case changes := <-ch:
			require.Equal(t, "new content", string(changes.Content))
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for clipboard change")
		}

		cancel()
		for range ch {
		}
	})

	t.Run("does not skip when the hint is not secret", func(t *testing.T) {
		const targets = "xclip -o -target TARGETS -selection clipboard -silent"
		const utf8 = "xclip -o -selection clipboard -target UTF8_STRING -silent"
		const hint = "xclip -o -selection clipboard -target x-kde-passwordManagerHint -silent"

		execer := NewMockExecer(t)
		execer.AddCall(targets, []byte("UTF8_STRING"))
		execer.AddCall(utf8, []byte("test"))
		execer.AddCall(targets, []byte("UTF8_STRING\nx-kde-passwordManagerHint"))
		execer.AddCallN(utf8, []byte("not a secret"), 2)
		execer.AddCall(hint, []byte("public"))

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
			WatchOptionWithFrequency(time.Millisecond),
			WatchOptionWithSkipSecrets(),
		)
		require.NoError(t, err)

		select {
		case changes := <-ch:
			require.Equal(t, "not a secret", string(changes.Content))
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
