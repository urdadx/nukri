// Package keys maps key presses to the actions they trigger.
package keys

type Action uint8

const (
	ActionCopy Action = iota + 1
	ActionCut
	ActionPaste
	ActionCancelClipboard
	ActionCreateArchive
	ActionExtractArchive
	ActionQuit
	ActionSearch
	ActionDown
	ActionUp
	ActionOpen
	ActionParent
	ActionHistoryBack
	ActionHistoryForward
	ActionNextPlace
	ActionPreviousPlace
	ActionPreviewPageDown
	ActionPreviewPageUp
	ActionToggleMark
	ActionTrash
	ActionDelete
	ActionCreate
	ActionRename // restores instead when browsing the trash
	ActionEditorRename
	ActionCopyPath
	ActionOpenDefault
	ActionOpenWith
	ActionFindDuplicates
	ActionThemes
	ActionTogglePreview
)

var defaults = map[string]Action{
	"y":            ActionCopy,
	"x":            ActionCut,
	"p":            ActionPaste,
	"esc":          ActionCancelClipboard,
	"C":            ActionCreateArchive,
	"e":            ActionExtractArchive,
	"q":            ActionQuit,
	"ctrl+c":       ActionQuit,
	"f":            ActionSearch,
	"down":         ActionDown,
	"j":            ActionDown,
	"up":           ActionUp,
	"k":            ActionUp,
	"enter":        ActionOpen,
	"backspace":    ActionParent,
	"left":         ActionParent,
	"h":            ActionParent,
	"alt+left":     ActionHistoryBack,
	"alt+right":    ActionHistoryForward,
	"tab":          ActionNextPlace,
	"shift+tab":    ActionPreviousPlace,
	"pgdown":       ActionPreviewPageDown,
	"ctrl+d":       ActionPreviewPageDown,
	"pgup":         ActionPreviewPageUp,
	"ctrl+u":       ActionPreviewPageUp,
	" ":            ActionToggleMark,
	"d":            ActionTrash,
	"delete":       ActionTrash,
	"D":            ActionDelete,
	"shift+delete": ActionDelete,
	"a":            ActionCreate,
	"r":            ActionRename,
	"f2":           ActionRename,
	"R":            ActionEditorRename,
	"shift+f2":     ActionEditorRename,
	"c":            ActionCopyPath,
	"o":            ActionOpenDefault,
	"O":            ActionOpenWith,
	"alt+d":        ActionFindDuplicates,
	"T":            ActionThemes,
	"P":            ActionTogglePreview,
	// bubbletea v1 reports Shift+Delete (CSI 3;2~) as alt+insert and
	// Shift+F2 (CSI 1;2Q) as f14.
	"alt+insert": ActionDelete,
	"f14":        ActionEditorRename,
}

// Resolve returns the action bound to key, as reported by bubbletea's
// KeyMsg.String().
func Resolve(key string) (Action, bool) {
	action, ok := defaults[key]
	return action, ok
}
