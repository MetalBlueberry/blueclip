package service

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnindent(t *testing.T) {
	type args struct {
		in io.Reader
	}
	tests := []struct {
		name    string
		args    args
		wantOut string
	}{
		{
			name: "unindent",
			args: args{
				in: strings.NewReader("  hello\n  world\n"),
			},
			wantOut: "hello\nworld\n",
		},
		{
			name: "unindent with tabs",
			args: args{
				in: strings.NewReader("		hello\n		world\n"),
			},
			wantOut: "hello\nworld\n",
		},
		{
			// this makes it easier to do the unindentation and
			// It should not make any functional difference
			name: "unindent adds new line at the end if missing",
			args: args{
				in: strings.NewReader("  hello\n  world"),
			},
			wantOut: "hello\nworld\n",
		},
		{
			name: "unindent uneven indentation",
			args: args{
				in: strings.NewReader(`    if (true) {
      return 1
    } else {
      return 0
    }
`),
			},
			wantOut: `if (true) {
  return 1
} else {
  return 0
}
`,
		},
		{
			name: "unindent uneven indentation with tabs",
			args: args{
				in: strings.NewReader(`				if (true) {
					return 1
				} else {
					return 0
				}
`),
			},
			wantOut: `if (true) {
	return 1
} else {
	return 0
}
`,
		},
		{
			name: "unindent never removes non indentation characters",
			args: args{
				in: strings.NewReader("  hello\nworld\n"),
			},
			wantOut: "hello\nworld\n",
		},
		{
			name: "unindent never removes characters in the middle or end",
			args: args{
				in: strings.NewReader("hello world  \n  as an example\n"),
			},
			wantOut: "hello world  \n  as an example\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			unindent(tt.args.in, out)
			require.Equal(t, tt.wantOut, out.String())
		})
	}
}
