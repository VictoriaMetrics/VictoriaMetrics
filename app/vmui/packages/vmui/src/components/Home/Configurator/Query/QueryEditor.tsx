import {EditorState} from "@codemirror/state";
import {EditorView, keymap} from "@codemirror/view";
import {defaultKeymap} from "@codemirror/commands";
import React, {FC, useEffect, useRef, useState} from "react";
import {PromQLExtension} from "codemirror-promql";
import {basicSetup} from "@codemirror/basic-setup";
import {QueryHistory} from "../../../../state/common/reducer";
import {ErrorTypes} from "../../../../types";

export interface QueryEditorProps {
    setHistoryIndex: (step: number, index: number) => void;
    setQuery: (query: string, index: number) => void;
    runQuery: () => void;
    query: string;
    index: number;
    queryHistory: QueryHistory;
    server: string;
    oneLiner?: boolean;
    autocomplete: boolean;
    error?: ErrorTypes | string;
}

const QueryEditor: FC<QueryEditorProps> = ({
  index,
  query,
  queryHistory,
  setHistoryIndex,
  setQuery,
  runQuery,
  server,
  oneLiner = false,
  autocomplete,
  error
}) => {

  const ref = useRef<HTMLDivElement>(null);

  const [editorView, setEditorView] = useState<EditorView>();
  const [focusEditor, setFocusEditor] = useState(false);

  // init editor view on load
  useEffect(() => {
    if (ref.current) {
      setEditorView(new EditorView(
        {
          parent: ref.current
        })
      );
    }
    return () => editorView?.destroy();
  }, []);

  // update state on change of autocomplete server
  useEffect(() => {
    const promQL = new PromQLExtension();
    promQL.activateCompletion(autocomplete);
    promQL.setComplete({remote: {url: server}});

    const listenerExtension = EditorView.updateListener.of(editorUpdate => {
      if (editorUpdate.focusChanged) {
        setFocusEditor(editorView?.hasFocus || false);
      }
      if (editorUpdate.docChanged) {
        setQuery(editorUpdate.state.doc.toJSON().map(el => el.trim()).join(""), index);
      }
    });

    editorView?.setState(EditorState.create({
      doc: query,
      extensions: [
        basicSetup,
        keymap.of(defaultKeymap),
        listenerExtension,
        promQL.asExtension(),
      ]
    }));
  }, [server, editorView, autocomplete, queryHistory]);

  const onKeyUp = (e: React.KeyboardEvent<HTMLDivElement>): void => {
    const {key, ctrlKey, metaKey} = e;
    const ctrlMetaKey = ctrlKey || metaKey;
    if (key === "Enter" && ctrlMetaKey) {
      runQuery();
    } else if (key === "ArrowUp" && ctrlMetaKey) {
      setHistoryIndex(-1, index);
    } else if (key === "ArrowDown" && ctrlMetaKey) {
      setHistoryIndex(1, index);
    }
  };

  return <div className={`query-editor-container 
    ${focusEditor ? "query-editor-container_focus" : ""}
    query-editor-container-${oneLiner ? "one-line" : "multi-line"}
    ${error === ErrorTypes.validQuery ? "query-editor-container_error" : ""}`}>
    {/*Class one-line-scroll and other codemirror styles are declared in index.css*/}
    <label className="query-editor-label">Query</label>
    <div className="query-editor" ref={ref} onKeyUp={onKeyUp}/>
  </div>;
};

export default QueryEditor;