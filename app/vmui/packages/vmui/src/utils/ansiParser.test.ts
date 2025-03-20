import { parseAnsiToHtml, getAnsiColor } from "./ansiParser";
import { render } from "@testing-library/preact";

describe("ANSI Parser", () => {
  // Test getAnsiColor for standard color values.
  test("getAnsiColor should return correct color for standard values", () => {
    expect(getAnsiColor(0)).toBe("#000000");
    expect(getAnsiColor(1)).toBe("#AA0000");
    expect(getAnsiColor(15)).toBe("#FFFFFF");
    expect(getAnsiColor(255)).toBe("rgb(238,238,238)");
  });

  // Test getAnsiColor for invalid color codes.
  test("getAnsiColor should return null for invalid color codes", () => {
    expect(getAnsiColor(-1)).toBeNull();
    expect(getAnsiColor(300)).toBeNull();
  });

  // Test that parseAnsiToHtml renders plain text without ANSI codes.
  test("parseAnsiToHtml should render text without ANSI codes correctly", () => {
    const { container } = render(parseAnsiToHtml("Hello, World!"));
    expect(container.textContent).toBe("Hello, World!");
  });

  // Test that parseAnsiToHtml applies correct foreground color.
  test("parseAnsiToHtml should apply correct styles for color codes", () => {
    const { container } = render(parseAnsiToHtml("\u001B[31mRed Text\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("color: #AA0000"); // Red color
  });

  // Test that parseAnsiToHtml resets styles when ANSI code 0 is used.
  test("parseAnsiToHtml should reset styles with ANSI code 0", () => {
    const { container } = render(parseAnsiToHtml("\u001B[31mRed \u001B[0mNormal"));
    const spans = container.querySelectorAll("span");

    expect(spans.length).toBe(2);
    expect(spans[0]).toHaveStyle("color: #AA0000");
    expect(spans[1]).toHaveStyle("color: inherit");
  });

  // Test that parseAnsiToHtml correctly parses bold text.
  test("parseAnsiToHtml should correctly parse bold text", () => {
    const { container } = render(parseAnsiToHtml("\u001B[1mBold Text\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("font-weight: bold");
  });

  // Test that parseAnsiToHtml correctly parses underlined text.
  test("parseAnsiToHtml should correctly parse underline text", () => {
    const { container } = render(parseAnsiToHtml("\u001B[4mUnderlined\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("text-decoration: underline");
  });

  // Test that parseAnsiToHtml correctly applies background colors.
  test("parseAnsiToHtml should correctly parse background colors", () => {
    const { container } = render(parseAnsiToHtml("\u001B[44mBlue Background\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("background-color: #0000AA");
  });

  // Edge case: Test that parseAnsiToHtml returns empty output for an empty input string.
  test("parseAnsiToHtml should return empty output for empty input", () => {
    const { container } = render(parseAnsiToHtml(""));
    expect(container.textContent).toBe("");
  });

  // Edge case: Test combined ANSI codes (e.g., bold and red text).
  test("parseAnsiToHtml should correctly parse combined ANSI codes", () => {
    const { container } = render(parseAnsiToHtml("\u001B[31;1mBold Red Text\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("color: #AA0000");
    expect(span).toHaveStyle("font-weight: bold");
  });

  // Edge case: Test extended foreground color using the ANSI sequence "38;5;n".
  test("parseAnsiToHtml should correctly parse extended foreground color", () => {
    // Using extended color code 82 for this test.
    const { container } = render(parseAnsiToHtml("\u001B[38;5;82mExtended Color\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle(`color: ${getAnsiColor(82)}`);
  });

  // Edge case: Test cancelling bold, italic, and underline styles.
  test("parseAnsiToHtml should correctly cancel bold, italic, and underline styles", () => {
    const input =
      "\u001B[1mBold\u001B[22m Normal " +
      "\u001B[3mItalic\u001B[23m Normal " +
      "\u001B[4mUnderline\u001B[24m Normal";
    const { container } = render(parseAnsiToHtml(input));
    const spans = container.querySelectorAll("span");

    // Check that after the cancellation codes, the style properties are set to 'inherit'
    spans.forEach(span => {
      if (span.textContent?.includes("Normal")) {
        expect(span).toHaveStyle("font-weight: inherit");
        expect(span).toHaveStyle("font-style: inherit");
        expect(span).toHaveStyle("text-decoration: inherit");
      }
    });
  });

  // Edge case: Test swapping foreground and background colors using codes 7 and 27.
  test("parseAnsiToHtml should correctly swap foreground and background colors", () => {
    // Set foreground to red (31) and background to blue (44), then swap with code 7.
    const { container } = render(parseAnsiToHtml("\u001B[31m\u001B[44m\u001B[7mSwapped Colors\u001B[0m"));
    const span = container.querySelector("span");
    // After swap, the color should be blue and the background should be red.
    expect(span).toHaveStyle("color: #0000AA");
    expect(span).toHaveStyle("background-color: #AA0000");
  });

  // Edge case: Test that unknown ANSI codes do not change the current styles.
  test("parseAnsiToHtml should ignore unknown ANSI codes", () => {
    const { container } = render(parseAnsiToHtml("\u001B[999mText with unknown code\u001B[0m"));
    const span = container.querySelector("span");
    expect(span).toHaveStyle("color: inherit");
  });
});
