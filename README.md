# Nukri

A fast and elegant terminal file manager for linux written in Go. Currently a work in progress

![alt text](demo.png)




## Keys

| Key | Action |
|---|---|
| `j` / `k` / `↑` / `↓` | Move down / up |
| `Enter` | Enter folder or open file |
| `h` / `←` / `Backspace` | Parent folder |
| `Alt+←` / `Alt+→` | Back / forward in history |
| `Tab` / `Shift+Tab` | Next / previous place |
| `Space` | Mark or unmark, then move down; actions apply to marked items, else the focused one |
| `Esc` | Clear marks, then the clipboard |
| `f` | Find in the current tree |
| `o` / `O` | Open with the default app / choose an app |
| `a` | Create files and folders (one per line; end with `/` for a folder) |
| `r` / `F2` | Rename (one line per marked item); in the trash, restore |
| `R` / `Shift+F2` | Rename in `$VISUAL` / `$EDITOR`; a path with `/` moves the item |
| `d` / `Delete` | Move to trash; in the trash, delete permanently |
| `D` / `Shift+Delete` | Delete permanently, after confirming |
| `c` | Copy name, name without extension, path or folder path |
| `y` / `x` / `p` | Copy / cut the marked items (or the focused one) / paste them all here |
| `C` / `e` | Create archive / extract archive |
| `Alt+D` | Find duplicate files |
| `T` | Pick a theme (previews live; Enter saves it) |
| `P` | Hide or show the preview |
| `PgDn` / `PgUp` | Scroll the preview |
| `q` | Quit |

The trash follows the FreeDesktop.org spec, so it's shared with your desktop's trash.
Copying uses `wl-copy`, `xclip` or `xsel` when available, and otherwise the terminal's
clipboard (OSC 52).

## Themes

Press `T` to pick a theme: moving through the list previews each one, Enter keeps it,
Esc puts the old one back. The choice is saved to `~/.config/nukri/config.toml`:

```toml
theme = "nord"
```

`nukri -theme <name>` overrides it for one run. To make your own, copy one of the
[bundled themes](nukri_config/themes) to `~/.config/nukri/themes/<name>.toml` and edit
it; a file there with a bundled theme's name replaces that theme.

## Optional tools for previews

Nukri doesn't bundle decoders for heavy file formats. It calls tools you already have
installed. If a preview needs a tool that's missing, the preview pane names the tool to
install. Everything else works without them.

| Preview | Tool | Debian / Ubuntu | Arch |
|---|---|---|---|
| Video frames, audio waveforms | `ffmpeg`, `ffprobe` | `ffmpeg` | `ffmpeg` |
| AVIF images | `magick` or `convert` (ffmpeg 6+ also works) | `imagemagick` | `imagemagick` |
| SVG images | `rsvg-convert` (best), or ImageMagick | `librsvg2-bin` | `librsvg` |
| PDF pages | `pdftocairo`, `pdfinfo` | `poppler-utils` | `poppler` |
| Office documents (docx, xlsx, pptx, odt, …) | `libreoffice` + the PDF tools | `libreoffice` | `libreoffice-fresh` |
| Ebooks (mobi, azw3) | `ebook-convert` + the PDF tools | `calibre` | `calibre` |
| `.tar.xz` / `.tar.zst` file lists | `xz` / `zstd` | `xz-utils` / `zstd` | `xz` / `zstd` |
| Faster listing of large `.tar.gz` | `pigz` (optional) | `pigz` | `pigz` |
| Other archives (7z, …) | `7z` or `7zz` | `7zip` | `7zip` |
| ISO disk images | `isoinfo` | `genisoimage` | `cdrtools` |
| Opening files with the default app | `xdg-open` | `xdg-utils` | `xdg-utils` |

EPUB metadata, fonts, torrents, images (PNG, JPEG, GIF, WebP, BMP, TIFF), code, Markdown,
CSV, ZIP and RAR archives, and tarballs (plain, gzip, bzip2) are previewed natively.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
