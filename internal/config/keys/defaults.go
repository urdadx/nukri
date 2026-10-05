package keys

var defaults = map[string]Action{
	"y":   ActionCopy,
	"x":   ActionCut,
	"p":   ActionPaste,
	"esc": ActionCancelClipboard,
	"C":   ActionCreateArchive,
	"e":   ActionExtractArchive,
}
