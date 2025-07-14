import { isMacOs } from "../../../../utils/detect-device";
import { DragIcon, SettingsIcon } from "../../../Main/Icons";

const metaKey = <code>{isMacOs() ? "Cmd" : "Ctrl"}</code>;

const graphTips = [
  {
    title: "Zoom in",
    description: <>
      To zoom in, hold down the {metaKey} + <code>scroll up</code>, or press the <code>+</code>.
      Also, you can zoom in on a range on the graph by holding down your mouse button and selecting the range.
    </>,
  },
  {
    title: "Zoom out",
    description: <>
      To zoom out, hold down the {metaKey} + <code>scroll down</code>, or press the <code>-</code>.
    </>,
  },
  {
    title: "Move horizontal axis",
    description: <>
      To move the graph, hold down the {metaKey} + <code>drag</code> the graph to the right or left.
    </>,
  },
  {
    title: "Fixing a tooltip",
    description: <>
      To fix the tooltip, <code>click</code> mouse when it&#39;s open.
      Then, you can drag the fixed tooltip by <code>clicking</code> and <code>dragging</code> on the <DragIcon/> icon.
    </>
  },
  {
    title: "Set a custom range for the vertical axis",
    description: <>
      To set a custom range for the vertical axis,
      click on the <SettingsIcon/> icon located in the upper right corner of the graph,
      activate the toggle, and set the values.
    </>
  },
];

const legendTips = [
  {
    title: "Show/hide a legend item",
    description: <>
      <code>click</code> on a legend item to isolate it on the graph.
      {metaKey} + <code>click</code> on a legend item to remove it from the graph.
      To revert to the previous state, click again.
    </>
  },
  {
    title: "Copy label key-value pairs",
    description: <>
      <code>click</code> on a label key-value pair to save it to the clipboard.
    </>
  },
  {
    title: "Collapse/Expand the legend group",
    description: <>
      <code>click</code> on the group name (e.g. <b>Query 1: &#123;__name__!=&#34;&#34;&#125;</b>)
      to collapse or expand the legend.
    </>
  },
];

export default graphTips.concat(legendTips);
