package xclip

type Selection struct {
	Content []byte
	Target  ValidTarget
}

func NewSelection(content []byte, t ValidTarget) Selection {
	return Selection{Content: content, Target: t}
}
