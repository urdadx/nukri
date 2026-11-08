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
)

var defaults = map[string]Action{
	"y":         ActionCopy,
	"x":         ActionCut,
	"p":         ActionPaste,
	"esc":       ActionCancelClipboard,
	"c":         ActionCreateArchive,
	"C":         ActionCreateArchive,
	"e":         ActionExtractArchive,
	"q":         ActionQuit,
	"ctrl+c":    ActionQuit,
	"f":         ActionSearch,
	"down":      ActionDown,
	"j":         ActionDown,
	"up":        ActionUp,
	"k":         ActionUp,
	"enter":     ActionOpen,
	"backspace": ActionParent,
	"left":      ActionParent,
	"h":         ActionParent,
	"alt+left":  ActionHistoryBack,
	"alt+right": ActionHistoryForward,
	"tab":       ActionNextPlace,
	"shift+tab": ActionPreviousPlace,
	"pgdown":    ActionPreviewPageDown,
	"ctrl+d":    ActionPreviewPageDown,
	"pgup":      ActionPreviewPageUp,
	"ctrl+u":    ActionPreviewPageUp,
}

// Resolve returns the action bound to key, as reported by bubbletea's
// KeyMsg.String().
func Resolve(key string) (Action, bool) {
	action, ok := defaults[key]
	return action, ok
}
