package selections

import (
	"blueclip/pkg/xclip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_Add_Single(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("test"),
			Type:    xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "test\n", buf.String())
}

func TestSet_Add_Multiple(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection A"),
			Type:    xclip.ValidTargetUTF8_STRING,
		},
	})

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Type:    xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection A\nSelection B\n", buf.String())
}

func TestSet_Incremental(t *testing.T) {
	s := NewSelections()
	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection"),
			Type:    xclip.ValidTargetUTF8_STRING,
		},
	})

	s.Add(Selection{
		Selection: xclip.Selection{
			Content: []byte("Selection B"),
			Type:    xclip.ValidTargetUTF8_STRING,
		},
	})

	buf := bytes.NewBuffer(nil)
	s.List(buf)
	require.Equal(t, "Selection B\n", buf.String())
}
