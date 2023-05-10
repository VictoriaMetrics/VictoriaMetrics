import React, { FC } from "preact/compat";
import Editor, { useMonaco } from "@monaco-editor/react";
import useMonacoTheme from "./hooks/useMonacoTheme";
import useLabelsSyntax from "./hooks/useLabelsSyntax";
import useKeybindings from "./hooks/useKeybindings";
import "./style.scss";

interface MonacoEditorProps {
  value: string;
  label?: string;
  language?: string;
  onChange: (val: string | undefined) => void;
  onEnter?: (val: string) => void;
}

const MonacoEditor: FC<MonacoEditorProps> = ({ value, label, language, onChange, onEnter }) => {
  const monaco = useMonaco();
  useMonacoTheme(monaco);
  useLabelsSyntax(monaco);
  useKeybindings(monaco, onEnter);

  return (
    <div className="vm-text-field vm-monaco-editor">
      <Editor
        className="vm-text-field__input vm-monaco-editor__input"
        defaultLanguage={language}
        value={value}
        theme={"vm-theme"}
        options={{
          scrollBeyondLastLine: false,
          automaticLayout: true,
          lineNumbers: "off",
          minimap: {
            enabled: false
          },
        }}
        onChange={onChange}
      />
      {label && <span className="vm-text-field__label">{label}</span>}
    </div>
  );
};

export default MonacoEditor;
