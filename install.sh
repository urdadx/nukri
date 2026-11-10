#!/bin/sh
# Install nukri, a terminal file manager for Linux.
#
#   curl -fsSL https://nukri.urdadx.com/install.sh | sh
#
# Settings, passed as environment variables to `sh`:
#   NUKRI_VERSION        version to install, such as 0.1.0 (default: latest)
#   NUKRI_INSTALL_DIR    where to put the binary (default: /usr/local/bin when
#                        writable, otherwise ~/.local/bin)
#   NUKRI_INSTALL_TOOLS  yes or no: install the optional preview tools without
#                        asking (default: ask when run from a terminal)
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
	install_menu_entry "$work/$archive"

	case ":$PATH:" in
	*":$dir:"*) say "Run it with: nukri" ;;
	*)
		say ""
		say "$dir is not on your PATH. Add it, for example in ~/.bashrc or ~/.zshrc:"
		say "  export PATH=\"$dir:\$PATH\""
		;;
	esac

	offer_preview_tools
}

# install_menu_entry ARCHIVE adds nukri, with its icon, to the desktop's app
# menu for this user. Releases before the entry existed have no such files, and
# a failure here never stops the install.
install_menu_entry() {
	tar -xzf "$1" -C "$work" packaging/nukri.desktop packaging/nukri.png 2>/dev/null || return 0
	data=${XDG_DATA_HOME:-$HOME/.local/share}
	mkdir -p "$data/applications" "$data/icons/hicolor/256x256/apps" 2>/dev/null &&
		cp "$work/packaging/nukri.desktop" "$data/applications/nukri.desktop" &&
		cp "$work/packaging/nukri.png" "$data/icons/hicolor/256x256/apps/nukri.png" || return 0
	# GTK trusts an icon-theme.cache over the folder while the folder's own time
	# is unchanged, so an icon added below an existing cache stays hidden until
	# the cache is rebuilt. Touching the folder makes GTK skip a stale cache when
	# the rebuild tool is missing.
	touch "$data/icons/hicolor" 2>/dev/null || true
	if has gtk-update-icon-cache; then
		gtk-update-icon-cache --quiet --force --ignore-theme-index "$data/icons/hicolor" 2>/dev/null || true
	fi
	if has update-desktop-database; then
		update-desktop-database --quiet "$data/applications" 2>/dev/null || true
	fi
	say "Added nukri to your app menu"
}

# Preview tools: "check|description|apt|pacman|dnf|zypper|apk". check lists
# commands, any of which means the tool is present. The packages match
# internal/preview/install_hint.go.
PREVIEW_TOOLS='ffmpeg ffprobe|video and audio|ffmpeg|ffmpeg|ffmpeg-free|ffmpeg|ffmpeg
rsvg-convert|SVG|librsvg2-bin|librsvg|librsvg2-tools|rsvg-convert|rsvg-convert
pdftoppm|PDF|poppler-utils|poppler|poppler-utils|poppler-tools|poppler-utils
magick convert|AVIF|imagemagick|imagemagick|ImageMagick|ImageMagick|imagemagick
7zz 7z|7z and other archives|7zip|7zip|7zip|7zip|7zip
xz|.tar.xz|xz-utils|xz|xz|xz|xz
zstd|.tar.zst|zstd|zstd|zstd|zstd|zstd
isoinfo|ISO images|genisoimage|cdrtools|genisoimage|genisoimage|cdrkit'
# Large and rarely needed, so only mentioned.
LARGE_TOOLS='libreoffice|office documents|libreoffice|libreoffice-fresh|libreoffice|libreoffice|libreoffice
ebook-convert|ebooks|calibre|calibre|calibre|calibre|calibre'

# package_manager prints "column|install command|assume-yes flag" for this
# system's package manager, or nothing.
package_manager() {
	if has apt-get; then printf '3|apt install|-y'
	elif has pacman; then printf '4|pacman -S|--noconfirm'
	elif has dnf; then printf '5|dnf install|-y'
	elif has zypper; then printf '6|zypper install|-y'
	elif has apk; then printf '7|apk add|'
	fi
}

# missing TOOLS COLUMN prints "package (description)" for each tool in TOOLS
# that isn't installed, one per line.
missing() {
	printf '%s\n' "$1" | while IFS= read -r line; do
		[ -n "$line" ] || continue
		found=no
		for command in $(printf '%s' "$line" | cut -d '|' -f 1); do
			has "$command" && found=yes
		done
		[ "$found" = yes ] && continue
		printf '%s (%s)\n' "$(printf '%s' "$line" | cut -d '|' -f "$2")" "$(printf '%s' "$line" | cut -d '|' -f 2)"
	done
}

# offer_preview_tools lists the preview tools that aren't installed and, with
# consent, installs them. nukri itself never installs anything; this asks
# once, here. It reads answers from the terminal, since under `curl | sh`
# standard input is the script.
offer_preview_tools() {
	manager=$(package_manager)
	column=$(printf '%s' "$manager" | cut -d '|' -f 1)
	install=$(printf '%s' "$manager" | cut -d '|' -f 2)
	assume_yes=$(printf '%s' "$manager" | cut -d '|' -f 3)
	if [ -z "$manager" ]; then
		say ""
		say "Some previews use external tools: ffmpeg, rsvg-convert, poppler, ImageMagick,"
		say "7-Zip, xz, zstd and isoinfo. Install the ones you want with your package manager."
		return
	fi
	tools=$(missing "$PREVIEW_TOOLS" "$column")
	large=$(missing "$LARGE_TOOLS" "$column")
	if [ -n "$tools" ]; then
		say ""
		say "These optional tools add previews:"
		printf '%s\n' "$tools" | sed 's/^/  /'
		packages=$(printf '%s\n' "$tools" | cut -d ' ' -f 1 | tr '\n' ' ')
		command="$install ${packages% }"
		[ "$(id -u)" = 0 ] || command="sudo $command"
		answer=${NUKRI_INSTALL_TOOLS:-}
		if [ -z "$answer" ] && (: </dev/tty) 2>/dev/null; then
			printf 'Install them now with "%s"? [y/N] ' "$command" >/dev/tty
			read -r answer </dev/tty || answer=no
		fi
		case $answer in
		y | Y | yes | YES)
			# From a terminal the package manager asks its own questions there;
			# without one, nobody could answer them, so it is told to proceed.
			if (: </dev/tty) 2>/dev/null; then
				run_install() { sh -c "$command" </dev/tty; }
			else
				run_install() { sh -c "$command $assume_yes" </dev/null; }
			fi
			if run_install; then
				say "Preview tools installed."
			else
				say "Installing the tools failed; you can run it yourself later:"
				say "  $command"
			fi
			;;
		*)
			say "Skipped. To add them later, run:"
			say "  $command"
			;;
		esac
	fi
	if [ -n "$large" ]; then
		say ""
		say "Large and optional, install if you want these previews:"
		printf '%s\n' "$large" | sed 's/^/  /'
	fi
}

# Everything runs from here, so a download cut off partway executes nothing.
main "$@"
