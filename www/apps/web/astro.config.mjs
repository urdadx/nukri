import node from "@astrojs/node";
// @ts-check
import tailwindcss from "@tailwindcss/vite";
import varlockAstroIntegration from "@varlock/astro-integration";
import { defineConfig } from "astro/config";

// https://astro.build/config
export default defineConfig({
  // Where the site lives, so absolute URLs such as og:image point there and not
  // at whatever host the request came through.
  site: "https://nukri.urdadx.com",
  integrations: [varlockAstroIntegration({ ssrInjectMode: "auto-load" })],
  output: "server",
  adapter: node({ mode: "standalone" }),
  vite: {
    plugins: [tailwindcss()],
  },
});
