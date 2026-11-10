/*
Nukri's 21 bundled themes for the demo. themes.json holds each theme's colors
after theme.Named has resolved them (filled in empty colors, made selected rows
readable), dumped from the Go code; regenerate it when configs/themes changes.
*/
import resolved from "./themes.json";
import type { Syntax, Theme } from "./render";

/*
Approximations of the chroma styles the themes name in code_syntax_highlight,
for the demo's small highlighter. Styles without an entry fall back to dracula.
*/
const palettes: Record<string, Syntax> = {
  dracula: {
    comment: "#6272a4",
    keyword: "#ff79c6",
    string: "#f1fa8c",
    number: "#bd93f9",
    name: "#8be9fd",
    func: "#50fa7b",
    heading: "#bd93f9",
  },
  monokai: {
    comment: "#75715e",
    keyword: "#f92672",
    string: "#e6db74",
    number: "#ae81ff",
    name: "#66d9ef",
    func: "#a6e22e",
    heading: "#f92672",
  },
  nord: {
    comment: "#616e88",
    keyword: "#81a1c1",
    string: "#a3be8c",
    number: "#b48ead",
    name: "#8fbcbb",
    func: "#88c0d0",
    heading: "#88c0d0",
  },
  gruvbox: {
    comment: "#928374",
    keyword: "#fb4934",
    string: "#b8bb26",
    number: "#d3869b",
    name: "#fabd2f",
    func: "#8ec07c",
    heading: "#fabd2f",
  },
  onedark: {
    comment: "#7f848e",
    keyword: "#c678dd",
    string: "#98c379",
    number: "#d19a66",
    name: "#e5c07b",
    func: "#61afef",
    heading: "#e06c75",
  },
  "rose-pine": {
    comment: "#6e6a86",
    keyword: "#31748f",
    string: "#f6c177",
    number: "#ebbcba",
    name: "#9ccfd8",
    func: "#ebbcba",
    heading: "#c4a7e7",
  },
  "ayu-dark": {
    comment: "#5c6773",
    keyword: "#ff8f40",
    string: "#aad94c",
    number: "#d2a6ff",
    name: "#59c2ff",
    func: "#ffb454",
    heading: "#ffb454",
  },
  evergarden: {
    comment: "#859289",
    keyword: "#e67e80",
    string: "#a7c080",
    number: "#d699b6",
    name: "#dbbc7f",
    func: "#83c092",
    heading: "#a7c080",
  },
  "catppuccin-mocha": {
    comment: "#6c7086",
    keyword: "#cba6f7",
    string: "#a6e3a1",
    number: "#fab387",
    name: "#f9e2af",
    func: "#89b4fa",
    heading: "#f38ba8",
  },
  "catppuccin-macchiato": {
    comment: "#6e738d",
    keyword: "#c6a0f6",
    string: "#a6da95",
    number: "#f5a97f",
    name: "#eed49f",
    func: "#8aadf4",
    heading: "#ed8796",
  },
  "catppuccin-frappe": {
    comment: "#737994",
    keyword: "#ca9ee6",
    string: "#a6d189",
    number: "#ef9f76",
    name: "#e5c890",
    func: "#8caaee",
    heading: "#e78284",
  },
  "catppuccin-latte": {
    comment: "#9ca0b0",
    keyword: "#8839ef",
    string: "#40a02b",
    number: "#fe640b",
    name: "#df8e1d",
    func: "#1e66f5",
    heading: "#d20f39",
  },
};

type Resolved = Omit<Theme, "syntax"> & { syntax: string };

export const themes: Record<string, Theme> = Object.fromEntries(
  Object.entries(resolved as Record<string, Resolved>).map(([name, theme]) => [
    name,
    { ...theme, syntax: palettes[theme.syntax] ?? palettes.dracula },
  ]),
);

export const themeNames = Object.keys(themes);
