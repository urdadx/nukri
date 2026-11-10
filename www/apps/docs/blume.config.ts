import { defineConfig } from "blume";

// Fonts are committed under ./fonts (latin subset, from Fontsource) so dev and
// build never fetch from Google Fonts.
const inter = {
  name: "Inter",
  variants: [
    { src: "./fonts/Inter-latin-wght-normal.woff2", weight: "100..900", style: "normal" },
    { src: "./fonts/Inter-latin-wght-italic.woff2", weight: "100..900", style: "italic" },
  ],
} as const;

export default defineConfig({
  title: "Nukri",
  description: "A fast terminal file manager for Linux, written in Go.",
  // The docs are served at nukri.urdadx.com/docs, beside the landing page
  // (apps/web), which serves this build's files from its own server.
  deployment: {
    site: "https://nukri.urdadx.com",
    base: "/docs",
  },
  // The same mark as the landing page (apps/web). The favicon and home screen
  // icon come from public/favicon.svg and public/apple-icon.png.
  logo: {
    image: "/logo.svg",
  },
  theme: {
    fonts: {
      display: inter,
      body: inter,
      mono: {
        name: "IBM Plex Mono",
        variants: [
          { src: "./fonts/IBMPlexMono-latin-400-normal.woff2", weight: 400, style: "normal" },
          { src: "./fonts/IBMPlexMono-latin-400-italic.woff2", weight: 400, style: "italic" },
          { src: "./fonts/IBMPlexMono-latin-500-normal.woff2", weight: 500, style: "normal" },
          { src: "./fonts/IBMPlexMono-latin-600-normal.woff2", weight: 600, style: "normal" },
        ],
      },
    },
  },
});
