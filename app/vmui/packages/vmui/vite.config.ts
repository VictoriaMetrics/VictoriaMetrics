import * as path from "path";

import { defineConfig } from "vite";
import preact from "@preact/preset-vite";
import dynamicIndexHtmlPlugin from "./config/plugins/dynamicIndexHtml";

export default defineConfig(({ mode }) => {
  return {
    base: "",
    plugins: [
      preact(),
      dynamicIndexHtmlPlugin({ mode })
    ],
    assetsInclude: ["**/*.md"],
    server: {
      open: true,
      port: 3000,
    },
    resolve: {
      alias: {
        "src": path.resolve(__dirname, "src"),
      },
    },
    build: {
      outDir: "./build",
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes("node_modules")) {
              return "vendor";
            }
          }
        }
      }
    },
  };
});


