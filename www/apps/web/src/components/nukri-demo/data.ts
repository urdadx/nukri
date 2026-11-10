import {
  formatSize,
  placeIcons,
  type Entry,
  type Line,
  type Place,
  type Preview,
  type Syntax,
  type Theme,
  type Visual,
} from "./render";

/** A made-up file system for the demo to browse. */
interface Node {
  name: string;
  modified: string;
  /** Present on folders. */
  children?: Node[];
  size?: number;
  /** Text shown in the preview pane, highlighted as lang. */
  text?: string;
  lang?: Lang;
  /** An image file's picture, at the pixel size the file claims to be. */
  image?: Visual;
  /** A video file's first frame and what ffprobe would report about it. */
  video?: Video;
  /** A PDF's first page and what pdfinfo would report about it. */
  pdf?: Pdf;
}

interface Pdf {
  page: string;
  width: number;
  height: number;
  title: string;
  author: string;
  creator: string;
  created: string;
  pages: number;
  pageSize: string;
}

interface Video {
  frame: string;
  duration: string;
  container: string;
  frameRate: string;
  bitRate: string;
}

type Lang = "Markdown" | "JSON" | "Go" | "Shell" | "Text" | "TOML" | "CSV";

const dir = (name: string, modified: string, children: Node[] = []): Node => ({
  name,
  modified,
  children,
});
const file = (name: string, size: number, modified: string, text?: string, lang?: Lang): Node => ({
  name,
  size,
  modified,
  text,
  lang,
});
// Photos from Unsplash, in public/demo (see CREDITS.md there). Images report
// the size of the photo they stand in for; the downloads are smaller.
const image = (
  name: string,
  size: number,
  modified: string,
  src: string,
  width: number,
  height: number,
): Node => ({
  name,
  size,
  modified,
  image: { src: `/demo/${src}`, width, height },
});
const video = (
  name: string,
  size: number,
  modified: string,
  frame: string,
  details: Omit<Video, "frame">,
): Node => ({
  name,
  size,
  modified,
  video: { frame: `/demo/${frame}`, ...details },
});
const pdf = (
  name: string,
  size: number,
  modified: string,
  page: string,
  details: Omit<Pdf, "page" | "width" | "height">,
  [width, height] = A4,
): Node => ({
  name,
  size,
  modified,
  pdf: { page: `/demo/${page}`, width, height, ...details },
});
// The pixel sizes of the drawn pages: A4 portrait and a 16:9 slide.
const A4: [number, number] = [420, 594];
const a4 = "595.276 x 841.89 pts (A4)";
const mkv = { container: "matroska,webm", frameRate: "30 fps", bitRate: "8000 kb/s" };
const mp4 = { container: "mov,mp4,m4a,3gp,3g2,mj2", frameRate: "29.97 fps", bitRate: "6000 kb/s" };

const bytes = (text: string) => new TextEncoder().encode(text).length;
const textFile = (name: string, modified: string, lang: Lang, text: string) =>
  file(name, bytes(text), modified, text, lang);

const readme = `# nukri

A fast and elegant terminal file manager for Linux.

## Install

    curl -fsSL https://nukri.urdadx.com/install.sh | sh

## Features

- Three panes: places, files and a live preview
- Rich previews for code, images, video, PDFs
- Copy, cut, paste, rename, trash and restore
- Fuzzy find across the current folder tree
- Duplicate finder, archives, drag and drop
- 21 themes, previewed live as you pick
`;

const mainGo = `package main

import (
	"fmt"
	"os"

	"github.com/urdadx/nukri/internal/ui"
)

// version is set at build time.
var version = ""

func main() {
	if err := ui.Run(version); err != nil {
		fmt.Fprintln(os.Stderr, "nukri:", err)
		os.Exit(1)
	}
}
`;

const goMod = `module github.com/urdadx/nukri

go 1.25

require (
	github.com/charmbracelet/bubbletea v1.3.4
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/BurntSushi/toml v1.5.0
)
`;

const packageJson = `{
  "name": "weekend-project",
  "version": "0.3.1",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest"
  },
  "dependencies": {
    "react": "19.2.0"
  }
}
`;

const todo = `Groceries
  - oat milk
  - coffee beans
  - lemons

This week
  - renew passport
  - call the landlord about the heater
  - back up the photos from the trip
  - finish chapter 4
`;

const budget = `month,rent,food,transport,fun
January,1200,410,95,120
February,1200,385,90,80
March,1200,402,110,150
April,1200,371,85,95
May,1200,420,100,210
June,1200,398,92,130
`;

const backup = `#!/bin/sh
# Back up the home folder to the external drive.
set -eu

target="/run/media/ada/backup"
if [ ! -d "$target" ]; then
	echo "backup drive not mounted" >&2
	exit 1
fi

rsync -a --delete --exclude node_modules \\
	"$HOME/" "$target/home/"
echo "done: $(date)"
`;

const nukriConfig = `# ~/.config/nukri/config.toml
theme = "dracula"
show_hidden = false
preview = true

[places]
show_devices = true
`;

const ideas = `# Ideas

- A tiny app that turns receipts into a budget
- Bike route planner that avoids hills
- Plant watering reminders, synced to the calendar
`;

const meeting = `# Planning, Nov 4

- Ship the preview cache before the release
- Write the changelog entry for themes
- Next sync: Thursday
`;

const home: Node[] = [
  dir("code", "3h ago", [
    dir("weekend-project", "3h ago", [
      dir("src", "3h ago", [
        file("main.tsx", 1_204, "3h ago"),
        file("App.tsx", 3_402, "3h ago"),
        file("index.css", 812, "5d ago"),
      ]),
      textFile("package.json", "3h ago", "JSON", packageJson),
      file("vite.config.ts", 211, "12d ago"),
    ]),
    dir("dotnet-sandbox", "4mo ago", [file("Program.cs", 640, "4mo ago")]),
    textFile("ideas.md", "2d ago", "Markdown", ideas),
  ]),
  dir("Desktop", "7mo ago"),
  dir("Documents", "1mo ago", [
    pdf("invoice-2026-10.pdf", 74_000, "9d ago", "invoice.jpg", {
      title: "Invoice 2026-10",
      author: "Ada Studio",
      creator: "Invoice Ninja",
      created: "Fri Oct 31 09:12:44 2026 WET",
      pages: 1,
      pageSize: a4,
    }),
    pdf("lease-agreement.pdf", 1_830_000, "8mo ago", "lease.jpg", {
      title: "Residential Lease Agreement",
      author: "Harbour Lettings",
      creator: "Microsoft Word",
      created: "Mon Mar  2 14:03:10 2026 WET",
      pages: 12,
      pageSize: a4,
    }),
    file("resume.docx", 107_900, "1mo ago"),
    file("taxes-2025.xlsx", 357_400, "6mo ago"),
    textFile("todo.txt", "1d ago", "Text", todo),
  ]),
  dir("dotfiles", "26d ago", [
    textFile("config.toml", "26d ago", "TOML", nukriConfig),
    file(".zshrc", 2_900, "26d ago"),
    file("gitconfig", 410, "3mo ago"),
  ]),
  dir("Downloads", "1h ago", [
    video("2026-08-15_16-20-52.mkv", 39_900_000, "3mo ago", "city-frame.jpg", {
      duration: "0:42",
      ...mkv,
    }),
    image("album-cover.png", 1_500_000, "14d ago", "album-cover.jpg", 3000, 2000),
    pdf("annual-report.pdf", 37_200, "1mo ago", "annual-report.jpg", {
      title: "Annual Report 2026",
      author: "Finance team",
      creator: "LibreOffice Writer",
      created: "Tue Oct  6 17:40:02 2026 WEST",
      pages: 28,
      pageSize: a4,
    }),
    file("backup-2026-11-01.tar.gz", 1_200_000_000, "9d ago"),
    pdf("boarding-pass.pdf", 74_000, "2mo ago", "boarding-pass.jpg", {
      title: "Boarding pass TP 662",
      author: "TAP Air Portugal",
      creator: "Check-in",
      created: "Wed Sep  9 06:58:21 2026 WEST",
      pages: 1,
      pageSize: a4,
    }),
    video("conference-talk.mp4", 150_600_000, "5d ago", "conference-frame.jpg", {
      duration: "24:16",
      ...mp4,
    }),
    textFile(
      "contacts.csv",
      "2mo ago",
      "CSV",
      "name,email\nAda,ada@example.com\nGrace,grace@example.com\n",
    ),
    file("fira-code-6.2.zip", 506_900, "4mo ago"),
    image("mountains-4k.jpg", 3_500_000, "21d ago", "mountains.jpg", 3840, 2560),
    file("nukri_0.2.0_linux_amd64.tar.gz", 6_300_000, "1h ago"),
    file("podcast-episode-12.mp3", 62_900_000, "6d ago"),
    pdf(
      "presentation.pdf",
      2_700_000,
      "3d ago",
      "presentation.jpg",
      {
        title: "Roadmap 2027",
        author: "Ada",
        creator: "Keynote",
        created: "Sat Nov  7 11:20:35 2026 WET",
        pages: 34,
        pageSize: "960 x 540 pts",
      },
      [640, 360],
    ),
    image("sunset.jpg", 1_500_000, "5mo ago", "sunset.jpg", 5347, 3565),
    file("synthwave-mix.flac", 32_000_000, "2mo ago"),
    file("ubuntu-26.04-desktop-amd64.iso", 6_100_000_000, "1mo ago"),
    file("wireframes.fig", 178_500, "14d ago"),
  ]),
  dir("go", "12mo ago", [
    dir("bin", "12mo ago", [file("gopls", 32_400_000, "12mo ago")]),
    dir("pkg", "12mo ago"),
  ]),
  dir("Music", "10mo ago", [
    file("track-01.flac", 31_200_000, "10mo ago"),
    file("track-02.flac", 28_700_000, "10mo ago"),
    file("track-03.flac", 35_100_000, "10mo ago"),
  ]),
  dir("node_modules", "27d ago", [
    dir("react", "27d ago"),
    dir("vite", "27d ago"),
    dir("vitest", "27d ago"),
  ]),
  dir("notes", "52m ago", [
    textFile("ideas.md", "52m ago", "Markdown", ideas),
    textFile("meeting-2026-11-04.md", "6d ago", "Markdown", meeting),
  ]),
  dir("Pictures", "25d ago", [
    dir("Screenshots", "2d ago", [
      image("screenshot-01.png", 412_000, "2d ago", "screenshot-01.jpg", 2560, 1600),
      image("screenshot-02.png", 390_100, "2d ago", "screenshot-02.jpg", 2560, 1600),
    ]),
    image("cat.jpg", 2_100_000, "25d ago", "cat.jpg", 5472, 3648),
  ]),
  dir("projects", "3h ago", [
    dir("nukri", "3h ago", [
      dir("cmd", "2d ago", [dir("nukri", "2d ago", [textFile("main.go", "2d ago", "Go", mainGo)])]),
      dir("configs", "7d ago", [dir("themes", "7d ago")]),
      dir("internal", "3h ago", [
        dir("preview", "3h ago"),
        dir("theme", "4d ago"),
        dir("ui", "3h ago"),
      ]),
      textFile("go.mod", "7d ago", "Go", goMod),
      textFile("README.md", "3h ago", "Markdown", readme),
      file("LICENSE", 1_067, "2mo ago"),
    ]),
  ]),
  dir("Public", "22mo ago"),
  dir("scripts", "22mo ago", [textFile("backup.sh", "22mo ago", "Shell", backup)]),
  dir("Templates", "22mo ago"),
  dir("tmp", "6mo ago", [file("scratch.txt", 233, "6mo ago")]),
  dir("Videos", "52m ago", [
    video("screen-recording.mp4", 51_700_000, "52m ago", "recording-frame.jpg", {
      duration: "1:54",
      ...mp4,
    }),
    video("holiday-2025.mov", 812_000_000, "14mo ago", "holiday-frame.jpg", {
      duration: "3:08",
      ...mp4,
    }),
  ]),
  dir("wallpapers", "5mo ago", [
    image("forest.jpg", 4_100_000, "5mo ago", "forest.jpg", 6000, 4000),
    image("dunes.png", 6_800_000, "5mo ago", "dunes.jpg", 3872, 2592),
  ]),
  textFile("budget.csv", "6mo ago", "CSV", budget),
  file("go1.25.3.linux-amd64.tar.gz", 59_800_000, "12mo ago"),
  textFile("README.md", "2d ago", "Markdown", readme),
  video("screen-recording.mp4", 51_700_000, "6h ago", "recording-frame.jpg", {
    duration: "1:54",
    ...mp4,
  }),
  textFile("todo.txt", "6mo ago", "Text", todo),
  dir(".local", "1d ago", [
    dir("share", "1d ago", [
      dir("Trash", "1d ago", [
        dir("files", "1d ago", [
          pdf("old-draft.pdf", 230_400, "1d ago", "draft.jpg", {
            title: "Chapter 4 (draft)",
            author: "Ada",
            creator: "LibreOffice Writer",
            created: "Sun Nov  8 22:47:13 2026 WET",
            pages: 9,
            pageSize: a4,
          }),
          file("untitled.txt", 0, "9d ago"),
        ]),
      ]),
    ]),
  ]),
];

const root = dir("/", "1mo ago", [
  dir("bin", "1mo ago"),
  dir("boot", "1mo ago"),
  dir("etc", "2d ago"),
  dir("home", "8mo ago", [dir("ada", "1h ago", home)]),
  dir("media", "3mo ago"),
  dir("opt", "4mo ago"),
  dir("tmp", "2m ago"),
  dir("usr", "1mo ago"),
  dir("var", "3h ago"),
]);

export const homePath = "/home/ada";

const placeList: { title: string; icon: string; path: string }[] = [
  { title: "Home", icon: placeIcons.home, path: homePath },
  { title: "Desktop", icon: placeIcons.desktop, path: `${homePath}/Desktop` },
  { title: "Documents", icon: placeIcons.documents, path: `${homePath}/Documents` },
  { title: "Downloads", icon: placeIcons.downloads, path: `${homePath}/Downloads` },
  { title: "Pictures", icon: placeIcons.pictures, path: `${homePath}/Pictures` },
  { title: "Music", icon: placeIcons.music, path: `${homePath}/Music` },
  { title: "Videos", icon: placeIcons.videos, path: `${homePath}/Videos` },
  { title: "Root", icon: placeIcons.root, path: "/" },
  { title: "Trash", icon: placeIcons.trash, path: `${homePath}/.local/share/Trash/files` },
];

export const placePaths = placeList.map((p) => p.path);

export const places = (cwd: string): Place[] =>
  placeList.map(({ title, icon, path }) => ({ title, icon, current: path === cwd }));

function find(path: string): Node | undefined {
  let node: Node | undefined = root;
  for (const part of path.split("/").filter(Boolean))
    node = node?.children?.find((c) => c.name === part);
  return node;
}

const byName = (a: Node, b: Node) =>
  Number(!a.children) - Number(!b.children) ||
  a.name.toLowerCase().localeCompare(b.name.toLowerCase());

/** The folder's entries as Nukri lists them: folders first, by name, hidden files left out. */
export function list(path: string): Entry[] {
  return (find(path)?.children ?? [])
    .filter((node) => !node.name.startsWith("."))
    .sort(byName)
    .map((node) => ({
      name: node.name,
      modified: node.modified,
      ...(node.children
        ? { items: node.children.filter((c) => !c.name.startsWith(".")).length }
        : { size: node.size }),
    }));
}

export const join = (parent: string, name: string) =>
  parent === "/" ? `/${name}` : `${parent}/${name}`;
export const parentOf = (path: string) => path.slice(0, path.lastIndexOf("/")) || "/";
export const isFolder = (path: string) => Boolean(find(path)?.children);

const kinds: Record<string, string> = {
  png: "Image",
  jpg: "Image",
  jpeg: "Image",
  mp4: "Video",
  mkv: "Video",
  mov: "Video",
  mp3: "Audio",
  flac: "Audio",
  pdf: "PDF document",
  zip: "Archive",
  gz: "Archive",
  iso: "ISO 9660 filesystem",
  docx: "Word document",
  xlsx: "Spreadsheet",
  fig: "File",
  tsx: "TypeScript",
  ts: "TypeScript",
  css: "CSS",
  cs: "C#",
};

export function preview(path: string, theme: Theme): Preview | undefined {
  const node = find(path);
  if (!node) return undefined;
  if (node.children) return { kind: "folder", entries: list(path) };
  if (node.image) {
    const { width, height } = node.image;
    return {
      kind: "file",
      title: "Image",
      detail: `${width}x${height}`,
      lines: [],
      visual: node.image,
    };
  }
  if (node.pdf) {
    // PDFPreview in internal/preview/view.go: the first page, then pdfinfo's fields in its order.
    const p = node.pdf;
    const fields: [string, string][] = [
      ["Title", p.title],
      ["Author", p.author],
      ["Creator", p.creator],
      ["Producer", "Skia/PDF m131"],
      ["CreationDate", p.created],
      ["Tagged", "no"],
      ["Pages", String(p.pages)],
      ["Encrypted", "no"],
      ["Page size", p.pageSize],
      ["File size", `${node.size} bytes`],
      ["PDF version", "1.7"],
    ];
    return {
      kind: "file",
      title: "PDF",
      detail: "First page",
      lines: fieldLines(fields, theme),
      visual: { src: p.page, width: p.width, height: p.height },
    };
  }
  if (node.video) {
    // VideoPreview in internal/preview/view.go: codec and size as the detail, then videoFields.
    const v = node.video;
    const fields: [string, string][] = [
      ["Duration", v.duration],
      ["Codec", "h264"],
      ["Container", v.container],
      ["Dimensions", "1920x1080"],
      ["Frame rate", v.frameRate],
      ["Pixel format", "yuv420p"],
      ["Bit rate", v.bitRate],
      ["Audio codec", "aac"],
      ["Audio tracks", "1"],
    ];
    return {
      kind: "file",
      title: "Video",
      detail: "h264 | 1920x1080",
      lines: fieldLines(fields, theme),
      visual: { src: v.frame, width: 1920, height: 1080 },
    };
  }
  if (node.text !== undefined) {
    return {
      kind: "file",
      title: node.lang ?? "Text",
      detail: "",
      lines: highlight(node.text, node.lang ?? "Text", theme),
    };
  }
  const ext = node.name.slice(node.name.lastIndexOf(".") + 1).toLowerCase();
  return {
    kind: "file",
    title: kinds[ext] ?? "File",
    detail: formatSize(node.size ?? 0),
    lines: [],
  };
}

/** fieldLines in internal/preview/view.go: names padded to line up their values. */
function fieldLines(fields: [string, string][], theme: Theme): Line[] {
  const pad = Math.max(...fields.map(([name]) => name.length));
  return fields.map(([name, value]) => [{ text: `${name.padEnd(pad)}  ${value}`, fg: theme.fg }]);
}

const rules: Partial<Record<Lang, [RegExp, keyof Syntax][]>> = {
  Go: [
    [/^\/\/.*/, "comment"],
    [/^"(?:[^"\\]|\\.)*"|^`[^`]*`/, "string"],
    [
      /^\b(?:package|import|func|var|const|type|return|if|else|for|range|go|defer|err|nil|require)\b/,
      "keyword",
    ],
    [/^\b(?:string|int|error|bool)\b/, "name"],
    [/^\b\d[\d.]*\b/, "number"],
    [/^[A-Za-z_]\w*(?=\()/, "func"],
  ],
  JSON: [
    [/^"(?:[^"\\]|\\.)*"(?=\s*:)/, "name"],
    [/^"(?:[^"\\]|\\.)*"/, "string"],
    [/^\b(?:true|false|null)\b/, "keyword"],
    [/^-?\d[\d.]*/, "number"],
  ],
  Shell: [
    [/^#.*/, "comment"],
    [/^"(?:[^"\\]|\\.)*"/, "string"],
    [/^\b(?:if|then|fi|else|set|exit|echo|for|do|done)\b/, "keyword"],
    [/^\$\w+|^\$\(/, "name"],
  ],
  TOML: [
    [/^#.*/, "comment"],
    [/^\[[^\]]*\]/, "keyword"],
    [/^"(?:[^"\\]|\\.)*"/, "string"],
    [/^\b(?:true|false)\b/, "number"],
    [/^[A-Za-z_][\w-]*(?=\s*=)/, "name"],
  ],
};

/** A small stand-in for chroma: enough to color the demo's files the way Nukri would. */
function highlight(text: string, lang: Lang, theme: Theme): Line[] {
  return text
    .replace(/\n$/, "")
    .split("\n")
    .map((source): Line => {
      const line = source.replace(/\t/g, "    ");
      const syntax = theme.syntax;
      if (lang === "Markdown") {
        if (line.startsWith("#")) return [{ text: line, fg: syntax.heading, bold: true }];
        if (/^\s*- /.test(line))
          return [
            { text: line.replace(/- .*/, "- "), fg: syntax.keyword },
            { text: line.replace(/^\s*- /, ""), fg: theme.fg },
          ];
        return [{ text: line, fg: /^ {4}/.test(line) ? syntax.string : theme.fg }];
      }
      const table = rules[lang];
      if (!table) return [{ text: line, fg: theme.fg }];
      const runs: Line = [];
      let rest = line;
      while (rest) {
        const hit = table
          .map(([re, kind]) => ({ match: re.exec(rest), fg: syntax[kind] }))
          .find((h) => h.match);
        if (hit?.match) {
          runs.push({ text: hit.match[0], fg: hit.fg });
          rest = rest.slice(hit.match[0].length);
        } else {
          const word = /^\w+|^./.exec(rest)![0];
          const last = runs.at(-1);
          if (last && last.fg === theme.fg) last.text += word;
          else runs.push({ text: word, fg: theme.fg });
          rest = rest.slice(word.length);
        }
      }
      return runs;
    });
}
