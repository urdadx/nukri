package keys

type Action uint8

const (
	ActionCopy Action = iota + 1
	ActionCut
	ActionPaste
	ActionCancelClipboard
	ActionCreateArchive
	ActionExtractArchive
)
