import { describe, expect, it } from "vitest";
import { markdownToSafeHtml, sanitizeHtml } from "./html";

const onErrorAttribute = "onerror";
const onMouseOverAttribute = "onmouseover";

describe("sanitizeHtml", () => {
  it("should remove scripts and event handlers", () => {
    const result = sanitizeHtml(`
      <script>alert(document.cookie)</script>
      <img src="data:," alt="test" ${onErrorAttribute}="alert(document.cookie)">
      <span ${onMouseOverAttribute}="alert(document.cookie)">text</span>
    `);

    expect(result).not.toContain("<script");
    expect(result).not.toContain("onerror");
    expect(result).not.toContain("onmouseover");
    expect(result).toContain("<img src=\"data:,\" alt=\"test\">");
    expect(result).toContain("<span>text</span>");
  });

  it("should preserve safe relabeling markup", () => {
    const value = "<span style=\"font-weight: bold; color: rgb(68, 149, 224);\" title=\"label\">metric_name</span>";

    expect(sanitizeHtml(value)).toBe(value);
  });

  it("should handle an empty string", () => {
    expect(sanitizeHtml("")).toBe("");
  });
});

describe("markdownToSafeHtml", () => {
  it("should render markdown and preserve its safe markup", () => {
    const result = markdownToSafeHtml("# Title\n\n**description**");

    expect(result).toContain("<h1>Title</h1>");
    expect(result).toContain("<strong>description</strong>");
  });

  it("should sanitize raw HTML rendered from markdown", () => {
    const result = markdownToSafeHtml(`<img src="data:," alt="test" ${onErrorAttribute}="alert(document.cookie)">`);

    expect(result).toContain("<img src=\"data:,\" alt=\"test\">");
    expect(result).not.toContain("onerror");
  });

  it("should remove unsafe link protocols", () => {
    const result = markdownToSafeHtml("[link](javascript:alert(document.cookie))");

    expect(result).toContain("link");
    expect(result).not.toContain("javascript:");
  });

  it("preserves allowed style properties", () => {
    const result = sanitizeHtml(
      "<span style=\"color:red;font-weight:bold\">text</span>",
    );

    expect(result).toContain("color: red");
    expect(result).toContain("font-weight: bold");
  });

  it("removes disallowed style properties", () => {
    const result = sanitizeHtml(
      "<span style=\"color:red;position:fixed;background-image:url(https://example.com)\">text</span>",
    );

    expect(result).toContain("color: red");
    expect(result).not.toContain("position");
    expect(result).not.toContain("background-image");
    expect(result).not.toContain("url(");
  });

  it("removes an empty style attribute", () => {
    const result = sanitizeHtml(
      "<span style=\"position:fixed\">text</span>",
    );

    expect(result).toBe("<span>text</span>");
  });

  it("removes style elements", () => {
    const result = sanitizeHtml(
      "<div>text<style>body { display: none }</style></div>",
    );

    expect(result).not.toContain("<style");
    expect(result).not.toContain("display");
  });
});
