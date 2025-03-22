package xclip

type Selection struct {
	Content []byte
	Type    ValidTarget
}

func NewSelection(content []byte, t ValidTarget) Selection {
	return Selection{Content: content, Type: t}
}
