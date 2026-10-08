package keys

var defaults = map[string]Action{
	"y":   ActionCopy,
	"x":   ActionCut,
	"p":   ActionPaste,
	"esc": ActionCancelClipboard,
	"c":   ActionCreateArchive,
	"C":   ActionCreateArchive,
	"e":   ActionExtractArchive,
}
