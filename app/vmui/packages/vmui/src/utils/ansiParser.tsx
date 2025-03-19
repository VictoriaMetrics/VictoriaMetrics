import React from "react";

// Define a specific interface for the ANSI style properties.
interface AnsiStyles {
  color: string | null;
  fontWeight: string | null;
  fontStyle: string | null;
  textDecoration: string | null;
  backgroundColor: string | null;
}

const getDefaultColors = (): Record<number, string> => {
  const colors: Record<number, string> = {
    0: "#000000", 1: "#AA0000", 2: "#00AA00", 3: "#AA5500",
    4: "#0000AA", 5: "#AA00AA", 6: "#00AAAA", 7: "#AAAAAA",
    8: "#555555", 9: "#FF5555", 10: "#55FF55", 11: "#FFFF55",
    12: "#5555FF", 13: "#FF55FF", 14: "#55FFFF", 15: "#FFFFFF"
  };

  for (let r = 0; r < 6; r++) {
    for (let g = 0; g < 6; g++) {
      for (let b = 0; b < 6; b++) {
        const index = 16 + (r * 36) + (g * 6) + b;
        const red = r > 0 ? r * 40 + 55 : 0;
        const green = g > 0 ? g * 40 + 55 : 0;
        const blue = b > 0 ? b * 40 + 55 : 0;
        colors[index] = `rgb(${red},${green},${blue})`;
      }
    }
  }

  for (let i = 0; i < 24; i++) {
    const index = 232 + i;
    const value = 8 + i * 10;
    colors[index] = `rgb(${value},${value},${value})`;
  }

  return colors;
};

const ansiColors = getDefaultColors();

const getAnsiColor = (code: number): string | null => ansiColors[code] || null;

const ST = "(?:\\u0007|\\u001B\\u005C|\\u009C)";
const ansiPattern = [
  `[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?${ST})`,
  "(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))"
].join("|");
const ansiRegex = new RegExp(ansiPattern, "g");

/**
 * Updates the style object based on a given ANSI code.
 *
 * @param styles - The current styles.
 * @param code - The ANSI code.
 * @param codes - All ANSI codes in the current escape sequence.
 * @returns The updated style object.
 */
const updateStyles = (
  styles: AnsiStyles,
  code: number,
  codes: number[]
): AnsiStyles => {
  switch (code) {
    case 0:
      // Reset all styles.
      return { color: null, fontWeight: null, fontStyle: null, textDecoration: null, backgroundColor: null };
    case 30: case 31: case 32: case 33:
    case 34: case 35: case 36: case 37:
      // Set foreground color.
      return { ...styles, color: getAnsiColor(code - 30) };
    case 90: case 91: case 92: case 93:
    case 94: case 95: case 96: case 97:
      // Set bright foreground color.
      return { ...styles, color: getAnsiColor(8 + (code - 90)) };
    case 38:
      // Extended foreground color: expects additional parameters, e.g., "38;5;{n}".
      if (codes.length > 2 && codes[1] === 5) {
        return { ...styles, color: getAnsiColor(codes[2]) };
      }
      return styles;
    case 40: case 41: case 42: case 43:
    case 44: case 45: case 46: case 47:
      // Set background color.
      return { ...styles, backgroundColor: getAnsiColor(code - 40) };
    case 100: case 101: case 102: case 103:
    case 104: case 105: case 106: case 107:
      // Set bright background color.
      return { ...styles, backgroundColor: getAnsiColor(8 + (code - 100)) };
    case 1:
      // Bold text.
      return { ...styles, fontWeight: "bold" };
    case 3:
      // Italic text.
      return { ...styles, fontStyle: "italic" };
    case 4:
      // Underline text.
      return { ...styles, textDecoration: "underline" };
    case 7: case 27:
      // Swap foreground and background colors.
      return { ...styles, color: styles.backgroundColor, backgroundColor: styles.color };
    case 22:
      // Normal intensity (cancel bold).
      return { ...styles, fontWeight: null };
    case 23:
      // Cancel italic.
      return { ...styles, fontStyle: null };
    case 24:
      // Cancel underline.
      return { ...styles, textDecoration: null };
    default:
      return styles;
  }
};

/**
 * Parses a string containing ANSI escape codes and returns an array of React elements with inline styles.
 *
 * @param input - The string to parse.
 * @returns An array of React.ReactNode elements.
 */
export const parseAnsiToHtml = (input: string): React.ReactNode[] => {
  let lastIndex = 0;
  const result: React.ReactNode[] = [];
  let currentStyles: AnsiStyles = {
    color: null,
    fontWeight: null,
    fontStyle: null,
    textDecoration: null,
    backgroundColor: null
  };

  let match;
  while ((match = ansiRegex.exec(input)) !== null) {
    // Process text before the ANSI escape sequence.
    const plainText = input.slice(lastIndex, match.index);
    if (plainText) {
      result.push(
        <span
          key={lastIndex}
          style={{
            color: currentStyles.color || "inherit",
            fontWeight: currentStyles.fontWeight || "inherit",
            fontStyle: currentStyles.fontStyle || "inherit",
            textDecoration: currentStyles.textDecoration || "inherit",
            backgroundColor: currentStyles.backgroundColor || "inherit"
          }}
        >
          {plainText}
        </span>
      );
    }

    // Extract ANSI codes from the escape sequence and update styles accordingly.
    const codes = match[0].match(/\d+/g)?.map(Number) || [];
    codes.forEach(code => {
      currentStyles = updateStyles(currentStyles, code, codes);
    });

    lastIndex = ansiRegex.lastIndex;
  }

  // Process any remaining text after the last ANSI escape sequence.
  if (lastIndex < input.length) {
    result.push(
      <span
        key={lastIndex}
        style={{
          color: currentStyles.color || "inherit",
          fontWeight: currentStyles.fontWeight || "inherit",
          fontStyle: currentStyles.fontStyle || "inherit",
          textDecoration: currentStyles.textDecoration || "inherit",
          backgroundColor: currentStyles.backgroundColor || "inherit"
        }}
      >
        {input.slice(lastIndex)}
      </span>
    );
  }

  return result;
};

export { ansiColors, getAnsiColor, getDefaultColors };
