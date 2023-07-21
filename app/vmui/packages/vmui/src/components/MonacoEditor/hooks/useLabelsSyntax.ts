import { useEffect } from "preact/compat";
import { Monaco } from "@monaco-editor/react";
import * as monaco from "monaco-editor";

const languageId = "vm-labels";

export const language = {
  ignoreCase: false,
  defaultToken: "",
  tokenizer: {
    root: [
      // labels
      [ /[a-z_]\w*(?=\s*(=|!=|=~|!~))/, "tag" ],

      // strings
      [ /"([^"\\]|\\.)*$/, "string.invalid" ],
      [ /'([^'\\]|\\.)*$/, "string.invalid" ],
      [ /"/, "string", "@string_double" ],
      [ /'/, "string", "@string_single" ],
      [ /`/, "string", "@string_backtick" ],

      // delimiters and operators
      [ /[{}()[]]/, "@brackets" ],
    ],

    string_double: [
      [ /[^\\"]+/, "string" ],
      [ /\\./, "string.escape.invalid" ],
      [ /"/, "string", "@pop" ]
    ],

    string_single: [
      [ /[^\\']+/, "string" ],
      [ /\\./, "string.escape.invalid" ],
      [ /'/, "string", "@pop" ]
    ],

    string_backtick: [
      [ /[^\\`$]+/, "string" ],
      [ /\\./, "string.escape.invalid" ],
      [ /`/, "string", "@pop" ]
    ],
  },
} as monaco.languages.IMonarchLanguage;

export const languageConfiguration: monaco.languages.LanguageConfiguration = {
  wordPattern: /(-?\d*\.\d\w*)|([^`~!#%^&*()\-=+[{\]}\\|;:'",.<>/?\s]+)/g,
  comments: {
    lineComment: "#",
  },
  brackets: [
    [ "{", "}" ],
    [ "[", "]" ],
    [ "(", ")" ],
  ],
  autoClosingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: "\"", close: "\"" },
    { open: "'", close: "'" },
  ],
  surroundingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: "\"", close: "\"" },
    { open: "'", close: "'" },
    { open: "<", close: ">" },
  ],
  folding: {}
};

const useLabelsSyntax = (monaco: Monaco | null) => {

  useEffect(() => {
    if (!monaco) return;
    monaco.languages.register({ id: languageId });
    monaco.languages.setMonarchTokensProvider(languageId, language);
    monaco.languages.setLanguageConfiguration(languageId, languageConfiguration);
  }, [monaco]);
};

export default useLabelsSyntax;
