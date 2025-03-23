package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSet_add_single(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("test"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "test\n", buf.String())
}

func TestSet_add_Multiple(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection B\nSelection A\n", buf.String())
}

func TestSet_incremental_filter_on_ephemeral(t *testing.T) {
	// When a new selection is added and there are exact matches in ephemeral list,
	// Those selections will be removed from ephemeral list.
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection B\n", buf.String())
}

func TestSet_incremental_filter_on_ephemeral_is_preserved_after(t *testing.T) {
	// If entries are added in decremental order, both are preserverd
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection\nSelection B\n", buf.String())
}

func TestSet_incremental_filter_ignores_important(t *testing.T) {
	// The incremental filter ignores important selections
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection", string(sel.Content))

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection C"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	assert.Equal(t, "Selection C\nSelection\nSelection B\n", buf.String())

	require.Len(t, s.Important, 1)
	assert.Equal(t, "Selection", string(s.Important[0].Content))
}

func TestSet_add_duplicate(t *testing.T) {
	// if a selection that exactly matches other is added, will be moved to the first place
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection A\nSelection B\n", buf.String())
}

func TestSet_add_duplicate_in_important(t *testing.T) {
	// When the duplicate is in the important list, it will be moved to the first place
	// and the other will be removed from the important list
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	selA, ok := s.Copy([]byte("Selection A\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection A", string(selA.Content))

	selB, ok := s.Copy([]byte("Selection B\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection B", string(selB.Content))

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	assert.Equal(t, "Selection A\nSelection B\n", buf.String())

	require.Len(t, s.Important, 2)
	assert.Equal(t, "Selection B", string(s.Important[0].Content))
	assert.Equal(t, "Selection A", string(s.Important[1].Content))
}

func TestSet_content_is_marked_as_important_when_copied(t *testing.T) {
	// When a selection is copied, it will be marked as important
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection C"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection D"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection A\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection A", string(sel.Content))

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	assert.Equal(t, "Selection A\nSelection D\nSelection C\nSelection B\n", buf.String())

	require.Len(t, s.Important, 1)
	assert.Equal(t, "Selection A", string(s.Important[0].Content))
}

func TestSet_last_selection_is_always_first(t *testing.T) {
	// Independently of being important or ephemeral, the last selection is always first
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection C"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection D"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection A\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection A", string(sel.Content))

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection E"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection F"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	assert.Equal(t, "Selection F\nSelection A\nSelection E\nSelection D\nSelection C\nSelection B\n", buf.String())

	require.Len(t, s.Important, 1)
	assert.Equal(t, "Selection A", string(s.Important[0].Content))
	require.Len(t, s.Ephemeral, 5)

	sel, ok = s.Copy([]byte("Selection B\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection B", string(sel.Content))

	buf = bytes.NewBuffer(nil)
	s.List(buf)
	assert.Equal(t, "Selection B\nSelection F\nSelection A\nSelection E\nSelection D\nSelection C\n", buf.String())

	require.Len(t, s.Important, 2)
	assert.Equal(t, "Selection A", string(s.Important[0].Content))
	assert.Equal(t, "Selection B", string(s.Important[1].Content))
	require.Len(t, s.Ephemeral, 4)
}

func TestSet_clear_ephemeral(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection B\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection B", string(sel.Content))

	s.Clear("Selection A\n", SelectionRetentionTypeEphemeral)

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection B\n", buf.String())

	require.Len(t, s.Important, 1)
	assert.Equal(t, "Selection B", string(s.Important[0].Content))
	require.Len(t, s.Ephemeral, 0)
	// Last selection is preserved
	require.Equal(t, "Selection B", string(s.Last.Content))
}

func TestSet_clear_important(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection B\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection B", string(sel.Content))

	s.Clear("Selection B\n", SelectionRetentionTypeImportant)

	require.Len(t, s.Important, 0)
	require.Len(t, s.Ephemeral, 1)
	assert.Equal(t, "Selection A", string(s.Ephemeral[0].Content))
	// Last selection is preserved
	require.Equal(t, "Selection B", string(s.Last.Content))
}

func TestSet_clear_all(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	sel, ok := s.Copy([]byte("Selection B\n"))
	assert.True(t, ok)
	assert.Equal(t, "Selection B", string(sel.Content))

	s.Clear("Selection B\n", SelectionRetentionTypeAll)

	require.Len(t, s.Important, 0)
	require.Len(t, s.Ephemeral, 1)
	assert.Equal(t, "Selection A", string(s.Ephemeral[0].Content))
	// Last selection is preserved
	require.Equal(t, "Selection B", string(s.Last.Content))

	s.Clear("Selection A\n", SelectionRetentionTypeAll)

	require.Len(t, s.Important, 0)
	require.Len(t, s.Ephemeral, 0)
	// Last selection is preserved
	require.Equal(t, "Selection B", string(s.Last.Content))

	// Last selection is gone after next selection is added
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection C"),
			Target:  xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection C\n", buf.String())

	require.Equal(t, "Selection C", string(s.Last.Content))
}
