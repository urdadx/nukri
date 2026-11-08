package keys_test

import (
	"testing"

	"github.com/urdadx/nukri/internal/config/keys"
)

func TestCreateArchiveKey(t *testing.T) {
	for _, key := range []string{"c", "C"} {
		action, ok := keys.Resolve(key)
		if !ok || action != keys.ActionCreateArchive {
			t.Fatalf("Resolve(%q) = %v, %v", key, action, ok)
		}
	}
}

func TestNavigationKeys(t *testing.T) {
	for key, want := range map[string]keys.Action{
		"j": keys.ActionDown, "down": keys.ActionDown,
		"k": keys.ActionUp, "enter": keys.ActionOpen,
		"h": keys.ActionParent, "backspace": keys.ActionParent,
		"q": keys.ActionQuit, "ctrl+c": keys.ActionQuit,
		"f": keys.ActionSearch, "shift+tab": keys.ActionPreviousPlace,
		"ctrl+d": keys.ActionPreviewPageDown, "pgup": keys.ActionPreviewPageUp,
	} {
		if action, ok := keys.Resolve(key); !ok || action != want {
			t.Errorf("Resolve(%q) = %v, %v; want %v", key, action, ok, want)
		}
	}
	if _, ok := keys.Resolve("z"); ok {
		t.Error("unbound key resolved to an action")
	}
}
