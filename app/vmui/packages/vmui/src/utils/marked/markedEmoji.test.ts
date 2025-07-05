import { marked } from "marked";
import markedEmoji from "./markedEmoji";
import { describe, expect, it } from "vitest";
import emojis from "../../constants/emojis";

describe("markedEmoji plugin", () => {
  marked.use(markedEmoji({ emojis, renderer: (token) => token.emoji }));
  const md = (src: string) => marked(src);

  it("replaces :smile: with emoji", () => {
    expect(md(":smile:")).toBe("<p>😄</p>\n");
  });

  it("replaces multiple emojis", () => {
    expect(md("Great job :thumbsup:!")).toBe("<p>Great job 👍!</p>\n");
  });

  it("leaves unknown emoji codes untouched", () => {
    expect(md("Hello :unknown:")).toBe("<p>Hello :unknown:</p>\n");
  });

  it("throws when emoji list is empty", () => {
    expect(() => markedEmoji({ emojis: {}, renderer: () => "" })).toThrow(
      /empty/i,
    );
  });

  it("works inside bold text", () => {
    expect(md("**Bold :smile:**")).toBe("<p><strong>Bold 😄</strong></p>\n");
  });

  it("works inside headings", () => {
    expect(md("# Heading :smile:")).toBe("<h1>Heading 😄</h1>\n");
  });

  it("works inside list items", () => {
    const src = "- item 1 :thumbsup:\n- item 2 :smile:";
    const expected = "<ul>\n<li>item 1 👍</li>\n<li>item 2 😄</li>\n</ul>\n";
    expect(md(src)).toBe(expected);
  });
});
