import { Monaco } from "@monaco-editor/react";
import { useEffect } from "preact/compat";
import * as monaco from "monaco-editor";

const useKeybindings = (monaco: Monaco | null, onEnter?: (val: string) => void) => {

  const handleRunEnter = (e: monaco.editor.ICodeEditor) => {
    onEnter && onEnter(e.getValue() || "");
  };

  useEffect(() => {
    if (!monaco) return;
    monaco.editor.addEditorAction({
      id: "execute-ctrl-enter",
      label: "Execute",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: handleRunEnter
    });
  }, [monaco, onEnter]);
};

export default useKeybindings;
