import React from "preact/compat";
import { isMacOs } from "../../../../utils/detect-device";
import { VisibilityIcon } from "../../Icons";
import GraphTips from "../../../Chart/GraphTips/GraphTips";

const ctrlMeta = <code>{isMacOs() ? "Cmd" : "Ctrl"}</code>;

export const AUTOCOMPLETE_QUICK_KEY = <>{<code>{isMacOs() ? "Option" : "Ctrl"}</code>} + <code>Space</code></>;

const keyList = [
  {
    title: "Query",
    list: [
      {
        keys: <code>Enter</code>,
        description: "Run"
      },
      {
        keys: <><code>Shift</code> + <code>Enter</code></>,
        description: "Multi-line queries"
      },
      {
        keys: <>{ctrlMeta} + <code>Arrow Up</code></>,
        description: "Previous command from the Query history"
      },
      {
        keys: <>{ctrlMeta} + <code>Arrow Down</code></>,
        description: "Next command from the Query history"
      },
      {
        keys: <>{ctrlMeta} + <code>click</code> by <VisibilityIcon/></>,
        description: "Toggle multiple queries"
      },
      {
        keys: AUTOCOMPLETE_QUICK_KEY,
        description: "Show quick autocomplete tips"
      }
    ]
  },
  {
    title: "Graph",
    readMore: <GraphTips/>,
    list: [
      {
        keys: <>{ctrlMeta} + <code>scroll Up</code> or <code>+</code></>,
        description: "Zoom in"
      },
      {
        keys: <>{ctrlMeta} + <code>scroll Down</code> or <code>-</code></>,
        description: "Zoom out"
      },
      {
        keys: <>{ctrlMeta} + <code>drag</code></>,
        description: "Move the graph left/right"
      },
      {
        keys: <><code>click</code></>,
        description: "Select the series in the legend"
      },
      {
        keys: <>{ctrlMeta} + <code>click</code></>,
        description: "Toggle multiple series in the legend"
      }
    ]
  },
];

export default keyList;
