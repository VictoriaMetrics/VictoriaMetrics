import react from "eslint-plugin-react";
import typescriptEslint from "@typescript-eslint/eslint-plugin";
import globals from "globals";
import tsParser from "@typescript-eslint/parser";
import path from "node:path";
import { fileURLToPath } from "node:url";
import js from "@eslint/js";
import { FlatCompat } from "@eslint/eslintrc";
import unusedImports from "eslint-plugin-unused-imports";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const compat = new FlatCompat({
  baseDirectory: __dirname,
  recommendedConfig: js.configs.recommended,
  allConfig: js.configs.all
});

export default [...compat.extends(
  "eslint:recommended",
  "plugin:react/recommended",
  "plugin:@typescript-eslint/recommended",
), {
  plugins: {
    react,
    "@typescript-eslint": typescriptEslint,
    "unused-imports": unusedImports,
  },

  languageOptions: {
    globals: {
      ...globals.browser,
    },

    parser: tsParser,
    ecmaVersion: 12,
    sourceType: "module",

    parserOptions: {
      ecmaFeatures: {
        jsx: true,
      },
    },
  },

  settings: {
    react: {
      pragma: "React",
      version: "19.0",
    },

    linkComponents: ["Hyperlink", {
      name: "Link",
      linkAttribute: "to",
    }],
  },

  rules: {
    "@typescript-eslint/no-unused-expressions": ["error", {
      allowShortCircuit: true,
      allowTernary: true
    }],

    "@typescript-eslint/no-unused-vars": ["error", {
      "argsIgnorePattern": "^_",
      "caughtErrors": "none",
      "caughtErrorsIgnorePattern": "^_",
      "destructuredArrayIgnorePattern": "^_",
      "varsIgnorePattern": "^_",
      "ignoreRestSiblings": true
    }],

    "unused-imports/no-unused-imports": "error",

    "react/jsx-closing-bracket-location": [1, "line-aligned"],
    "object-curly-spacing": [2, "always"],

    "react/jsx-max-props-per-line": [1, {
      maximum: 1,
    }],

    "react/jsx-first-prop-new-line": [1, "multiline"],

    // Disable core indent rule due to recursion issues in ESLint 9; use JSX-specific rules instead
    indent: ["error", 2, {
      SwitchCase: 1,
      ignoredNodes: [
        "JSXElement",
        "JSXElement *",
        "JSXFragment",
        "JSXFragment *",
      ],
    }],
    "react/jsx-indent": ["error", 2],
    "react/jsx-indent-props": ["error", 2],

    "linebreak-style": ["error", "unix"],
    quotes: ["error", "double"],
    semi: ["error", "always"],
    // Formatting rules moved out of ESLint core; omit here to avoid deprecation noise
    "react/prop-types": 0,
    "react/react-in-jsx-scope": "off",
  },
}];
