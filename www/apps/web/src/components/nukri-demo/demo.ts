/*
The demo's state and the actions Nukri binds to keys and the mouse
(internal/config/keys): j/k or the arrows move, enter opens, h, left or
backspace go to the parent, P toggles the preview, and clicking a place or a
file selects it, with a double click to open.
*/
import { homePath, isFolder, join, list, parentOf, placePaths, places, preview } from "./data";
import { render, type Frame } from "./render";
import { themeNames, themes } from "./themes";

/** The footer status until the demo has focus: how to drive it. */
export const hint = "click to try · j/k move · enter open · h back · T theme";
/** The same for touch screens, which have no keys to press. */
export const touchHint = "tap to explore · double tap opens";

/** How many rows tall the demo's terminal is. */
export const rows = 32;

export class Demo {
  theme = themes.dracula;
  cwd = homePath;
  selected = 4; // Downloads
  hidePreview = false;
  status = hint;

  constructor(
    public cols: number,
    public rows: number,
    themeName = "dracula",
  ) {
    this.theme = themes[themeName] ?? themes.dracula;
  }

  private get entries() {
    return list(this.cwd);
  }

  private get current() {
    return this.entries[this.selected];
  }

  frame(): Frame {
    const entry = this.current;
    return render(
      {
        cols: this.cols,
        rows: this.rows,
        cwd: this.cwd,
        places: places(this.cwd),
        entries: this.entries,
        selected: this.selected,
        preview: entry ? preview(join(this.cwd, entry.name), this.theme) : undefined,
        hidePreview: this.hidePreview,
        status: this.status,
      },
      this.theme,
    );
  }

  move(by: number) {
    const count = this.entries.length;
    if (count) this.selected = Math.min(count - 1, Math.max(0, this.selected + by));
  }

  go(path: string, select = 0) {
    this.cwd = path;
    this.selected = select;
  }

  open() {
    const entry = this.current;
    if (!entry) return;
    const path = join(this.cwd, entry.name);
    if (isFolder(path)) this.go(path);
  }

  parent() {
    if (this.cwd === "/") return;
    const from = this.cwd.slice(this.cwd.lastIndexOf("/") + 1);
    const parent = parentOf(this.cwd);
    this.go(
      parent,
      Math.max(
        0,
        list(parent).findIndex((e) => e.name === from),
      ),
    );
  }

  togglePreview() {
    this.hidePreview = !this.hidePreview;
  }

  /** Steps to the next bundled theme, saying so in the status line as Nukri does. */
  nextTheme() {
    const current = themeNames.findIndex((name) => themes[name] === this.theme);
    const name = themeNames[(current + 1) % themeNames.length];
    this.theme = themes[name];
    this.status = `Theme: ${name}`;
  }

  /** Handles a key the way Nukri does; false when the key isn't one of its bindings. */
  key(key: string): boolean {
    switch (key) {
      case "j":
      case "ArrowDown":
        this.move(1);
        break;
      case "k":
      case "ArrowUp":
        this.move(-1);
        break;
      case "Enter":
        this.open();
        break;
      case "h":
      case "ArrowLeft":
      case "Backspace":
        this.parent();
        break;
      case "P":
        this.togglePreview();
        break;
      // Nukri's T opens a theme picker; here it steps through the themes.
      case "T":
      case "t":
        this.nextTheme();
        return true;
      default:
        return false;
    }
    this.status = "Ready";
    return true;
  }

  /** A click on cell (x, y); double opens what was clicked. False when nothing is there. */
  click(x: number, y: number, double: boolean): boolean {
    const frame = this.frame();
    const inside = (a: Frame["files"]) =>
      x >= a.x && x < a.x + a.width && y >= a.y && y < a.y + a.height;
    if (frame.sidebar && inside(frame.sidebar)) {
      const path = placePaths[y - frame.sidebar.y];
      if (path === undefined) return false;
      if (path !== this.cwd) this.go(path);
    } else if (inside(frame.files)) {
      const index = frame.files.start + y - frame.files.y;
      if (index >= this.entries.length) return false;
      this.selected = index;
      if (double) this.open();
    } else {
      return false;
    }
    this.status = "Ready";
    return true;
  }
}
