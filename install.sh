#!/bin/sh
# Install nukri, a terminal file manager for Linux.
#
#   curl -fsSL https://nukri.urdadx.com/install.sh | sh
#
# Settings, passed as environment variables to `sh`:
#   NUKRI_VERSION      version to install, such as 0.1.0 (default: latest)
#   NUKRI_INSTALL_DIR  where to put the binary (default: /usr/local/bin when
#                      writable, otherwise ~/.local/bin)
#
#   curl -fsSL https://nukri.urdadx.com/install.sh | NUKRI_VERSION=0.1.0 sh
#
# The download is checked against the release's checksums.txt before anything
# is installed.

set -eu

REPO="urdadx/nukri"

say() { printf '%s\n' "$*"; }
fail() { printf 'nukri install: %s\n' "$*" >&2; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

# download URL FILE: fetch URL into FILE with curl or wget.
download() {
	if has curl; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif has wget; then
		wget -q -O "$2" "$1"
	else
		fail "curl or wget is required"
	fi
}

# latest_version prints the newest release's version without its "v", read
# from GitHub's release redirect rather than its rate-limited API.
latest_version() {
	url="https://github.com/$REPO/releases/latest"
	if has curl; then
		location=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$url")
	else
		location=$(wget -q -S --spider "$url" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1)
	fi
	tag=${location##*/tag/}
	tag=$(printf '%s' "$tag" | tr -d '\r')
	[ -n "$tag" ] && [ "$tag" != "$location" ] || fail "could not find the latest release"
	printf '%s' "${tag#v}"
}

detect_arch() {
	case $(uname -m) in
	x86_64 | amd64) printf 'amd64' ;;
	aarch64 | arm64) printf 'arm64' ;;
	*) fail "unsupported CPU $(uname -m); releases are built for x86_64 and arm64" ;;
	esac
}

# sha256 FILE prints FILE's SHA-256 checksum.
sha256() {
	if has sha256sum; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif has shasum; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		fail "sha256sum or shasum is required to verify the download"
	fi
}

install_dir() {
	if [ -n "${NUKRI_INSTALL_DIR:-}" ]; then
		printf '%s' "$NUKRI_INSTALL_DIR"
	elif [ -w /usr/local/bin ]; then
		printf '/usr/local/bin'
	else
		printf '%s/.local/bin' "$HOME"
	fi
}

main() {
	[ "$(uname -s)" = Linux ] || fail "nukri runs on Linux only"
	arch=$(detect_arch)
	version=${NUKRI_VERSION:-$(latest_version)}
	version=${version#v}
	archive="nukri_${version}_linux_${arch}.tar.gz"
	base="https://github.com/$REPO/releases/download/v$version"

	work=$(mktemp -d)
	trap 'rm -rf "$work"' EXIT INT TERM

	say "Downloading nukri $version for linux/$arch..."
	download "$base/$archive" "$work/$archive" || fail "could not download $archive (does version $version exist?)"
	download "$base/checksums.txt" "$work/checksums.txt" || fail "could not download checksums.txt"

	expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$work/checksums.txt")
	[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
	[ "$(sha256 "$work/$archive")" = "$expected" ] || fail "checksum mismatch for $archive; not installing"

	tar -xzf "$work/$archive" -C "$work" nukri || fail "the archive has no nukri binary"

	dir=$(install_dir)
	mkdir -p "$dir" || fail "could not create $dir"
	# Install through a temporary name, so a running nukri is replaced whole.
	cp "$work/nukri" "$dir/.nukri.new" || fail "could not write to $dir"
	chmod 755 "$dir/.nukri.new"
	mv -f "$dir/.nukri.new" "$dir/nukri"
	say "Installed nukri $version to $dir/nukri"

	case ":$PATH:" in
	*":$dir:"*) say "Run it with: nukri" ;;
	*)
		say ""
		say "$dir is not on your PATH. Add it, for example in ~/.bashrc or ~/.zshrc:"
		say "  export PATH=\"$dir:\$PATH\""
		;;
	esac
}

# Everything runs from here, so a download cut off partway executes nothing.
main "$@"
