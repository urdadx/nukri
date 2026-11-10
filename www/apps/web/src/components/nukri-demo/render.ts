/*
A port of Nukri's browser view (internal/ui/browser and internal/ui/chrome.go)
that draws into a grid of terminal cells instead of ANSI text, so the landing
page can show the real interface in HTML. The layout, padding, columns and
colors follow the Go code; when that changes, change this to match.
*/
import table from "./icons.json";

/**
 * The colors the browser view uses, from a resolved theme.Theme as
 * internal/ui/styles.go maps it. The *Bg colors are left out when a theme
 * draws selected rows on the panel's own background.
 */
export interface Theme {
  bg: string;
  fg: string;
  sidebarFg: string;
  border: string;
  activeBorder: string;
  path: string;
  /** Place icons in the sidebar, and the icon color in folder previews. */
  directory: string;
  /** The generic folder icon in the file list. */
  folder: string;
  selected: string;
  selectedBg?: string;
  sidebarSelected: string;
  sidebarSelectedBg?: string;
  muted: string;
  /** Colors for highlighted code previews. */
  syntax: Syntax;
}

export interface Syntax {
  comment: string;
  keyword: string;
  string: string;
  number: string;
  name: string;
  func: string;
  heading: string;
}

export interface Entry {
  name: string;
  /** Item count for a folder, which marks the entry as a folder. */
  items?: number;
  /** Size in bytes for a file. */
  size?: number;
  /** Already formatted, as formatModified would print it. */
  modified: string;
}

export interface Place {
  title: string;
  icon: string;
  current?: boolean;
}

/** A run of text in one style. Icons are runs of their own so they can be held to one cell. */
export interface Run {
  text: string;
  fg: string;
  bg?: string;
  bold?: boolean;
  icon?: boolean;
}

export type Line = Run[];

export type Preview =
  | { kind: "folder"; entries: Entry[] }
  | { kind: "file"; title: string; detail: string; lines: Line[]; visual?: Visual };

/** A picture shown in the preview: an image, or a video's frame. Width and height are its pixels. */
export interface Visual {
  src: string;
  width: number;
  height: number;
}

/** A picture placed on the grid, in cells. */
export interface Picture {
  x: number;
  y: number;
  cols: number;
  rows: number;
  src: string;
}

export interface Screen {
  cols: number;
  rows: number;
  cwd: string;
  places: Place[];
  entries: Entry[];
  selected: number;
  preview?: Preview;
  hidePreview?: boolean;
  status: string;
}

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
  color: string;
}

/** Where a pane's rows are, for mapping clicks back to entries and places. */
export interface Area {
  x: number;
  y: number;
  width: number;
  height: number;
  /** Index of the item drawn on the area's first row. */
  start: number;
}

export interface Frame {
  lines: Line[];
  boxes: Box[];
  pictures: Picture[];
  sidebar?: Area;
  files: Area;
}

// Place icons and the generic folder from internal/config/icons/function.go.
export const placeIcons = {
  home: "\u{f02dc}",
  desktop: "\u{f01c4}",
  documents: "\u{f0219}",
  downloads: "\u{f03d4}",
  pictures: "\u{f02e9}",
  music: "♬",
  videos: "\u{f0381}",
  root: "\u{f02ca}",
  trash: "\u{f1f8}",
};
const folderIcon = "\u{f07b}";
const openFolderIcon = "\u{f0256}";

// browser.PanelContentTop: the top border, the title and a blank line.
const panelContentTop = 3;

type Style = { icon: string; color: string };
const icons = table.icons as Record<string, Style>;
const aliases = table.aliases as Record<string, string>;
const folders = table.folders as Record<string, Style>;

/** iconFor in internal/ui/load.go. */
export function iconFor(entry: Entry, theme: Theme): Style {
  if (entry.items !== undefined) {
    return folders[entry.name] ?? { icon: folderIcon, color: theme.folder };
  }
  const name = entry.name.toLowerCase();
  if (icons[name]) return icons[name];
  let key = name;
  if (aliases[key]) {
    key = aliases[key];
  } else {
    const dot = name.lastIndexOf(".");
    key = dot > 0 ? name.slice(dot + 1) : "";
    key = aliases[key] ?? key;
  }
  return icons[key] ?? icons.file;
}

const width = (line: Line) => line.reduce((sum, run) => sum + [...run.text].length, 0);

/** Cuts text to width cells, ending in "…" when it doesn't fit, like ansi.Truncate. */
function truncate(text: string, cells: number): string {
  const chars = [...text];
  if (chars.length <= cells) return text;
  return chars.slice(0, Math.max(0, cells - 1)).join("") + "…";
}

function truncateLine(line: Line, cells: number, tail = ""): Line {
  if (width(line) <= cells) return line;
  const out: Line = [];
  let left = cells - [...tail].length;
  for (const run of line) {
    const chars = [...run.text];
    if (chars.length <= left) {
      out.push(run);
      left -= chars.length;
      continue;
    }
    const text = chars.slice(0, Math.max(0, left)).join("") + tail;
    if (text) out.push({ ...run, text });
    break;
  }
  return out;
}

function pad(line: Line, cells: number, fg: string, bg?: string): Line {
  const gap = cells - width(line);
  return gap > 0 ? [...line, { text: " ".repeat(gap), fg, bg }] : line;
}

const padEnd = (text: string, cells: number) =>
  text + " ".repeat(Math.max(0, cells - [...text].length));

export function formatSize(size: number): string {
  if (size < 1000) return `${size} B`;
  const units = ["kB", "MB", "GB", "TB"];
  let value = size;
  for (const unit of units) {
    value /= 1000;
    if (value < 1000 || unit === units.at(-1)) return `${value.toFixed(1)} ${unit}`;
  }
  return `${size} B`;
}

const detailFor = (entry: Entry) =>
  entry.items !== undefined ? `${entry.items} items` : formatSize(entry.size ?? 0);

interface Layout {
  sidebar: number;
  files: number;
  preview: number;
  filesHeight: number;
  previewHeight: number;
  stacked: boolean;
}

/** ResolveLayout in internal/ui/browser/layout.go. */
function resolveLayout(cols: number, rows: number, showPreview: boolean): Layout {
  const none = { preview: 0, previewHeight: 0, stacked: false };
  if (cols < 34) return { sidebar: 0, files: cols, filesHeight: rows, ...none };
  let sidebar = 23;
  if (cols < 104) sidebar -= Math.trunc((104 - cols) / 4);
  if (sidebar < 11) sidebar = 5;
  const remaining = cols - sidebar;
  if (!showPreview) return { sidebar, files: remaining, filesHeight: rows, ...none };
  if (cols <= 54) {
    const filesHeight = Math.trunc((rows * 54) / 100);
    if (filesHeight < 6) return { sidebar, files: remaining, filesHeight: rows, ...none };
    return {
      sidebar,
      files: remaining,
      preview: remaining,
      filesHeight,
      previewHeight: rows - filesHeight,
      stacked: true,
    };
  }
  const files = Math.trunc((remaining * 54) / 100);
  const preview = remaining - files;
  if (files < 18 || preview < 14) return { sidebar, files: remaining, filesHeight: rows, ...none };
  return { sidebar, files, preview, filesHeight: rows, previewHeight: rows, stacked: false };
}

/**
 * panel: a rounded border with one cell of padding each side and, when titled,
 * the title and a blank line above the content. The border is returned as a
 * Box; its cells are left blank.
 */
function panel(
  content: Line[],
  title: string,
  cols: number,
  rows: number,
  theme: Theme,
  border: string,
) {
  const inner = Math.max(0, cols - 4);
  let lines = content;
  if (title) lines = [[{ text: title, fg: theme.path, bold: true }], [], ...lines];
  const blank: Line = [{ text: " ".repeat(cols), fg: theme.fg }];
  const out: Line[] = [blank];
  for (let row = 0; row < rows - 2; row++) {
    const line = truncateLine(lines[row] ?? [], inner);
    out.push([
      { text: "  ", fg: theme.fg },
      ...pad(line, inner, theme.fg),
      { text: "  ", fg: theme.fg },
    ]);
  }
  out.push(blank);
  return { lines: out.slice(0, rows), border };
}

function renderSidebar(cols: number, rows: number, screen: Screen, theme: Theme) {
  const compact = cols <= 5;
  const rowWidth = Math.max(1, cols - 4);
  const content = screen.places.map((place): Line => {
    const fg = place.current ? theme.sidebarSelected : theme.sidebarFg;
    const bg = place.current ? theme.sidebarSelectedBg : undefined;
    const bold = place.current;
    return pad(
      [
        { text: " ", fg, bg, bold },
        { text: place.icon, fg: theme.directory, bg, bold, icon: true },
        { text: compact ? "" : "  " + place.title, fg, bg, bold },
      ],
      rowWidth,
      fg,
      bg,
    );
  });
  return panel(content, compact ? "" : " Places ", cols, rows, theme, theme.border);
}

const listStart = (selected: number, visible: number) =>
  selected >= visible ? selected - visible + 1 : 0;

function renderList(cols: number, rows: number, screen: Screen, theme: Theme): Line[] {
  if (screen.entries.length === 0) return [[{ text: "This folder is empty", fg: theme.muted }]];
  const nameColumn = Math.max(1, cols - 27);
  const start = listStart(screen.selected, rows);
  return screen.entries.slice(start, start + rows).map((entry, offset): Line => {
    const { icon, color } = iconFor(entry, theme);
    let nameWidth = cols - 4;
    let metadata = "";
    if (cols >= 34) {
      nameWidth = nameColumn;
      metadata = `${detailFor(entry).padStart(10)} ${entry.modified.padStart(9)}`;
    }
    const nameText =
      " " + padEnd(truncate(entry.name, Math.max(1, nameWidth)), Math.max(1, nameWidth));
    if (start + offset === screen.selected) {
      const style = { fg: theme.selected, bg: theme.selectedBg, bold: true };
      const content = truncateLine(
        [
          { text: icon, ...style, icon: true },
          { text: nameText + metadata, ...style },
        ],
        cols - 2,
      );
      return pad([{ text: "▌ ", ...style }, ...content], cols, theme.selected, theme.selectedBg);
    }
    return pad(
      [
        { text: "  ", fg: theme.sidebarFg },
        { text: icon, fg: color === "NONE" ? theme.fg : color, icon: true },
        { text: nameText + metadata, fg: theme.sidebarFg },
      ],
      cols,
      theme.fg,
    );
  });
}

function renderEntries(cols: number, rows: number, screen: Screen, theme: Theme) {
  const title = ` ${openFolderIcon}  ${truncate(screen.cwd, Math.max(1, cols - 10))} `;
  return panel(
    renderList(cols - 4, rows - 4, screen, theme),
    title,
    cols,
    rows,
    theme,
    theme.activeBorder,
  );
}

function renderPreviewEntries(entries: Entry[], body: number, theme: Theme): Line[] {
  return entries.map((item, index): Line => {
    const style = iconFor(item, theme);
    const detail = detailFor(item);
    const name = truncate(item.name, Math.max(1, body - detail.length - 5));
    const gap = " ".repeat(Math.max(1, body - 1 - [...name].length - detail.length - 2));
    if (index === 0) {
      const selected = { fg: theme.selected, bg: theme.selectedBg, bold: true };
      return pad(
        [
          { text: style.icon, ...selected, icon: true },
          { text: ` ${name}${gap}${detail}`, ...selected },
        ],
        body,
        theme.selected,
        theme.selectedBg,
      );
    }
    return [
      { text: " ", fg: theme.sidebarFg },
      { text: style.icon, fg: style.color === "NONE" ? theme.directory : style.color, icon: true },
      { text: ` ${name}${gap}${detail}`, fg: theme.sidebarFg },
    ];
  });
}

/**
 * fitImageToPane in internal/ui/browser/visual.go: the largest grid of cells
 * that fits the pane and keeps the picture's proportions, taking a cell to be
 * twice as tall as it is wide.
 */
function fitImageToPane(
  width: number,
  height: number,
  maxCols: number,
  maxRows: number,
): [number, number] {
  const cellAspect = 2;
  const scale = Math.min(maxCols / width, (maxRows * cellAspect) / height);
  if (!(scale > 0)) return [0, 0];
  return [
    Math.max(1, Math.trunc(width * scale)),
    Math.max(1, Math.trunc((height * scale) / cellAspect)),
  ];
}

function renderPreview(cols: number, rows: number, screen: Screen, theme: Theme) {
  const body = Math.max(1, cols - 4);
  const muted = (text: string): Line => [{ text, fg: theme.muted }];
  const entry = screen.entries[screen.selected];
  let title = " Preview ";
  let text: Line[] = [muted("Nothing selected")];
  let visual: Omit<Picture, "x" | "y"> | undefined;
  if (entry) {
    title = ` ${iconFor(entry, theme).icon}  ${entry.name} `;
    const preview = screen.preview;
    if (preview?.kind === "folder") {
      text = preview.entries.length
        ? renderPreviewEntries(preview.entries, body, theme)
        : [muted("Folder is empty")];
    } else if (preview?.kind === "file") {
      const head = [preview.title, preview.detail].filter(Boolean).join(" • ");
      text = [muted(head), [], ...preview.lines.map((line) => truncateLine(line, body, "…"))];
      if (preview.visual) {
        // The picture goes at the top, with the text below it.
        const [width, height] = fitImageToPane(
          preview.visual.width,
          preview.visual.height,
          body,
          Math.max(1, rows - 4),
        );
        visual = { cols: width, rows: height, src: preview.visual.src };
        text = [...Array.from({ length: height }, () => []), ...text];
      }
    }
  }
  const drawn = panel(
    text.slice(0, Math.max(0, rows - 4)),
    truncate(title, Math.max(1, cols - 6)),
    cols,
    rows,
    theme,
    theme.border,
  );
  return { ...drawn, visual };
}

function renderFooter(cols: number, screen: Screen, theme: Theme): Line {
  const entry = screen.entries[screen.selected];
  const position = entry ? `${screen.selected + 1}/${screen.entries.length}` : "0/0";
  const left: Run = { text: ` ${position}  ${entry?.name ?? ""}`, fg: theme.path, bold: true };
  const right: Run = { text: `${screen.status}  q quit `, fg: theme.muted };
  const room = Math.max(0, cols - width([left]));
  const rightRun = { ...right, text: [...right.text].slice(0, room).join("") };
  const gap = Math.max(0, cols - width([left, rightRun]));
  return truncateLine([left, { text: " ".repeat(gap), fg: theme.fg }, rightRun], cols);
}

/** Render in internal/ui/browser/browser.go plus the footer: rows of runs and the borders to draw over them. */
export function render(screen: Screen, theme: Theme): Frame {
  const body = screen.rows - 1;
  const layout = resolveLayout(screen.cols, body, !screen.hidePreview);
  type Pane = { x: number; y: number; cols: number; rows: number; lines: Line[]; border: string };
  const panes: Pane[] = [];
  const pictures: Picture[] = [];
  if (layout.sidebar) {
    panes.push({
      x: 0,
      y: 0,
      cols: layout.sidebar,
      rows: body,
      ...renderSidebar(layout.sidebar, body, screen, theme),
    });
  }
  const filesX = layout.sidebar;
  panes.push({
    x: filesX,
    y: 0,
    cols: layout.files,
    rows: layout.filesHeight,
    ...renderEntries(layout.files, layout.filesHeight, screen, theme),
  });
  if (layout.preview) {
    const x = layout.stacked ? filesX : filesX + layout.files;
    const y = layout.stacked ? layout.filesHeight : 0;
    const { visual, ...preview } = renderPreview(
      layout.preview,
      layout.previewHeight,
      screen,
      theme,
    );
    panes.push({ x, y, cols: layout.preview, rows: layout.previewHeight, ...preview });
    // Inside the border and padding, below the title and its blank line.
    if (visual) pictures.push({ x: x + 2, y: y + panelContentTop, ...visual });
  }

  const grid: Line[][] = Array.from({ length: body }, () => []);
  for (const pane of panes) {
    pane.lines.forEach((line, row) => grid[pane.y + row].push(line.length ? line : []));
  }
  const lines = grid.map((parts) => parts.flat());
  lines.push(renderFooter(screen.cols, screen, theme));

  const listRows = layout.filesHeight - 4;
  return {
    lines,
    boxes: panes.map((p) => ({ x: p.x, y: p.y, width: p.cols, height: p.rows, color: p.border })),
    pictures,
    sidebar:
      layout.sidebar > 5
        ? { x: 0, y: panelContentTop, width: layout.sidebar, height: body - 4, start: 0 }
        : undefined,
    files: {
      x: filesX,
      y: panelContentTop,
      width: layout.files,
      height: listRows,
      start: listStart(screen.selected, listRows),
    },
  };
}

const escape = (text: string) =>
  text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

/** The frame as HTML for the component's screen element, on the server and in the browser alike. */
export function toHTML(frame: Frame): string {
  const rows = frame.lines
    .map((line) => {
      const runs = line
        .filter((run) => run.text)
        .map((run) => {
          const classes = [run.bold && "b", run.icon && "i"].filter(Boolean).join(" ");
          const style = run.bg ? `color:${run.fg};background:${run.bg}` : `color:${run.fg}`;
          return `<span${classes ? ` class="${classes}"` : ""} style="${style}">${escape(run.text)}</span>`;
        })
        .join("");
      return `<div class="row">${runs}</div>`;
    })
    .join("");
  const boxes = frame.boxes
    .map(
      (b) =>
        `<div class="box" style="--x:${b.x};--y:${b.y};--w:${b.width};--h:${b.height};border-color:${b.color}"></div>`,
    )
    .join("");
  const pictures = frame.pictures
    .map(
      (p) =>
        `<img class="pic" src="${p.src}" alt="" style="--x:${p.x};--y:${p.y};--w:${p.cols};--h:${p.rows}">`,
    )
    .join("");
  return rows + boxes + pictures;
}
