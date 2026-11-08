# Nukri

A fast and elegant terminal file manager for linux written in Go. Currently a work in progress

![alt text](demo.png)




## Optional tools for previews

Nukri doesn't bundle decoders for heavy file formats. It calls tools you already have
installed. If a preview needs a tool that's missing, the preview pane names the tool to
install. Everything else works without them.

| Preview | Tool | Debian / Ubuntu | Arch |
|---|---|---|---|
| Video frames, audio waveforms | `ffmpeg`, `ffprobe` | `ffmpeg` | `ffmpeg` |
| AVIF images | `magick` or `convert` (ffmpeg 6+ also works) | `imagemagick` | `imagemagick` |
| SVG images | `rsvg-convert` | `librsvg2-bin` | `librsvg` |
| PDF pages | `pdftocairo`, `pdfinfo` | `poppler-utils` | `poppler` |
| Office documents (docx, xlsx, pptx, odt, …) | `libreoffice` + the PDF tools | `libreoffice` | `libreoffice-fresh` |
| Ebooks (mobi, azw3) | `ebook-convert` + the PDF tools | `calibre` | `calibre` |
| Archives other than ZIP (7z, rar, tar, …) | `7z` or `7zz` | `7zip` | `7zip` |
| ISO disk images | `isoinfo` | `genisoimage` | `cdrtools` |
| Opening files with the default app | `xdg-open` | `xdg-utils` | `xdg-utils` |

EPUB metadata, fonts, torrents, images (PNG, JPEG, GIF, WebP, BMP, TIFF), code, Markdown,
CSV and ZIP archives are previewed natively.
