# Nukri

A fast terminal file manager for Linux, written in Go. Nukri is a work in progress.

![Nukri with the Places sidebar, the file list of a home folder, and a preview of the selected folder](demo.png)

## Install

With Go installed:

```bash
go install github.com/urdadx/nukri/cmd/nukri@latest
```

This puts `nukri` in `~/go/bin`. Icons need a [Nerd Font](https://www.nerdfonts.com/).

## Features

- **Three panes**: places and mounted devices, the file list, and a live preview. The
  layout adapts to the terminal's size, and `P` hides the preview.
- **Rich previews**: syntax-highlighted code, Markdown, CSV, images (kitty graphics or
  sixel), video frames, audio waveforms, PDF pages, office documents, ebooks, fonts,
  torrents, ISO images, and the contents of folders and archives.
- **File operations**: mark several items, then copy, cut and paste, create, rename
  inline or in your editor, trash and restore, or delete permanently.
- **Archives**: create zip, tar and tar.gz archives, and extract them.
- **Fuzzy find** across the current folder tree.
- **Duplicate finder** that shows how much space you can reclaim.
- **Drag and drop** in kitty 0.47+, in both directions, including over SSH.
- **Open** files with their default app or a chosen one, and copy names or paths to the
  clipboard.
- **21 themes**, previewed live as you pick, plus your own.
- **Mouse support**: click, double-click and scroll.

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
