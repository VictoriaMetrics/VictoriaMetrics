import { useEffect } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import { Monaco } from "@monaco-editor/react";

const useMonacoTheme = (monaco: Monaco | null) => {
  const { isDarkTheme } = useAppState();

  useEffect(() => {
    if (!monaco) return;
    monaco.editor.defineTheme("vm-theme", {
      base: isDarkTheme ? "vs-dark" : "vs",
      inherit: true,
      rules: [],
      colors: {
        // #00000000 - for transparent
        "editor.background": "#00000000",
        "editor.lineHighlightBackground": "#00000000",
        "editor.lineHighlightBorder": "#00000000"
      }
    });
    monaco.editor.setTheme("vm-theme");
  }, [monaco, isDarkTheme]);
};

export default useMonacoTheme;
