import {EditorState} from "@codemirror/next/state";
import {EditorView, keymap} from "@codemirror/next/view";
import {defaultKeymap} from "@codemirror/next/commands";
import React, {FC, useEffect, useRef, useState} from "react";
import { PromQLExtension } from "codemirror-promql";
import { basicSetup } from "@codemirror/next/basic-setup";

export interface QueryEditorProps {
  setQuery: (query: string) => void;
  query: string;
  server: string;
  oneLiner?: boolean;
}

const QueryEditor: FC<QueryEditorProps> = ({query, setQuery, server, oneLiner = false}) => {

  const ref = useRef<HTMLDivElement>(null);

  const [editorView, setEditorView] = useState<EditorView>();

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

    const promQL = new PromQLExtension().setComplete({url: server});

    const listenerExtension = EditorView.updateListener.of(editorUpdate => {
      if (editorUpdate.docChanged) {
        setQuery(
          editorUpdate.state.doc.toJSON().map(el => el.trim()).join("")
        );
      }

    });

    editorView?.setState(EditorState.create({
      doc: query,
      extensions: [basicSetup, keymap(defaultKeymap), listenerExtension, promQL.asExtension()]
    }));

  }, [server, editorView]);

  return (
    <>
      {/*Class one-line-scroll and other codemirror stylings are declared in index.css*/}
      <div ref={ref} className={oneLiner ? "one-line-scroll" : undefined}></div>
    </>
  );
};

export default QueryEditor;