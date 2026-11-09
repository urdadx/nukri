package kittydnd

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// machineIDKey is the HMAC key the protocol hashes machine ids with.
const machineIDKey = "tty-dnd-protocol-machine-id"

/*
MachineID returns this machine's id in the protocol's form, or "" when it is
unknown. The terminal compares it with its own to tell whether nukri runs on
another machine, as over SSH, and if so moves file contents through the
terminal instead of passing paths.
*/
func MachineID() string {
	raw := rawMachineID()
	if raw == "" {
		return ""
	}
	return HashMachineID(raw)
}

// HashMachineID turns a raw machine id into the protocol's form: version 1,
// then the hex HMAC-SHA256 of the id.
func HashMachineID(raw string) string {
	mac := hmac.New(sha256.New, []byte(machineIDKey))
	mac.Write([]byte(raw))
	return "1:" + hex.EncodeToString(mac.Sum(nil))
}

var platformUUID = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

// rawMachineID reads the id the protocol names for this platform: the
// IOPlatformUUID on macOS, /etc/machine-id elsewhere.
func rawMachineID() string {
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err != nil {
			return ""
		}
		if match := platformUUID.FindSubmatch(output); match != nil {
			return strings.TrimRight(string(match[1]), " \t\n\r\v\f")
		}
		return ""
	}
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), " \t\n\r\v\f") // as kitty does
}
