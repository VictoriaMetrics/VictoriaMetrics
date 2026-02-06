import * as path from "path";

import { defineConfig, ProxyOptions } from "vite";
import preact from "@preact/preset-vite";

const getProxy = (): Record<string, ProxyOptions> | undefined => {
  const playground = process.env.PLAYGROUND?.toLowerCase();

  if (playground !== "true") {
    return undefined;
  }

  const commonProxy: ProxyOptions = {
    target: "https://play.victoriametrics.com/select/0",
    changeOrigin: true,
    configure: (proxy) => {
      proxy.on("error", (err) => {
        console.error("[proxy error]", err.message);
      });
    },
  };

  return {
    "^/prometheus/(api|vmalert)/.*": { ...commonProxy },
    "/prometheus/vmui/config.json": { ...commonProxy },
  };
};

export default defineConfig(() => {
  return {
    base: "",
    plugins: [preact()],
    assetsInclude: ["**/*.md"],
    server: {
      open: true,
      port: 3000,
      proxy: getProxy(),
    },
    resolve: {
      alias: {
        src: path.resolve(__dirname, "src"),
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
          },
        },
      },
    },
  };
});
