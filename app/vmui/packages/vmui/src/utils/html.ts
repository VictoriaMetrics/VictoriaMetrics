import DOMPurify, { type Config } from "dompurify";
import { marked } from "marked";

const HTML_SANITIZE_CONFIG: Config = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ["style"],
};

const ALLOWED_STYLE_PROPERTIES = new Set([
  "color",
  "background-color",

  "font-family",
  "font-size",
  "font-style",
  "font-weight",
  "font-variant",
  "font-stretch",
  "line-height",

  "letter-spacing",
  "word-spacing",

  "text-align",
  "text-decoration",
  "text-decoration-color",
  "text-decoration-line",
  "text-decoration-style",
  "text-transform",
  "text-shadow",

  "white-space",
  "word-break",
  "overflow-wrap",
  "text-overflow",
  "vertical-align",
]);

DOMPurify.addHook("uponSanitizeAttribute", (node, hookEvent) => {
  if (hookEvent.attrName.toLowerCase() !== "style") {
    return;
  }

  // Use the browser CSS parser to normalize declarations and safely
  // handle comments, escaping and malformed CSS.
  const parsedStyle = node.ownerDocument.createElement("span").style;
  parsedStyle.cssText = hookEvent.attrValue;

  // Iterate backwards because removeProperty changes the collection.
  for (let index = parsedStyle.length - 1; index >= 0; index--) {
    const property = parsedStyle.item(index).toLowerCase();

    if (!ALLOWED_STYLE_PROPERTIES.has(property)) {
      parsedStyle.removeProperty(property);
    }
  }

  hookEvent.attrValue = parsedStyle.cssText;
  hookEvent.keepAttr = parsedStyle.length > 0;
});

export const sanitizeHtml = (value: string): string => {
  return DOMPurify.sanitize(value, HTML_SANITIZE_CONFIG);
};

export const markdownToSafeHtml = (value: string): string => {
  return sanitizeHtml(marked.parse(value) as string);
};
