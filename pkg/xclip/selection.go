package xclip

type Selection struct {
	Content []byte
	Target  ValidTarget
	// Source is the X11 selection the content was read from. It is empty for
	// entries saved before it was recorded.
	Source ClipboardSelection
}

func NewSelection(content []byte, t ValidTarget) Selection {
	return Selection{Content: content, Target: t}
}
