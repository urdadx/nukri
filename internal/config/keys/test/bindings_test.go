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
