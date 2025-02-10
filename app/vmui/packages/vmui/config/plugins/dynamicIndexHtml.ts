import { readFile } from "fs/promises";
import { IndexHtmlTransform } from "vite";

/**
 * Vite plugin to dynamically load index.html based on the current mode.
 * If a specific mode-based index file (e.g., index.victorialogs.html) exists, it is used.
 * Otherwise, the default index.html is loaded.
 */
export default function dynamicIndexHtmlPlugin({ mode }) {
  return {
    name: "vm-dynamic-index-html",
    transformIndexHtml: {
      order: "pre",
      handler: async () => {
        try {
          return await readFile(`./index.${mode}.html`, "utf8");
        } catch (error) {
          return await readFile("./index.html", "utf8");
        }
      }
    } as IndexHtmlTransform
  };
}
