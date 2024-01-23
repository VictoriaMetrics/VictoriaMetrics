import React from "preact/compat";
import { DATE_FILENAME_FORMAT } from "../../../constants/date";
import router, { routerOptions } from "../../../router";
import { Link } from "react-router-dom";

const filename = (
  <>
    <p>Filename - specify the name for your report file.</p>
    <p>Default format: <code>vmui_report_${DATE_FILENAME_FORMAT}.json</code>.</p>
    <p>This name will be used when saving your report on your device.</p>
  </>
);

const comment = (
  <>
    <p>Comment (optional) - add a comment to your report.</p>
    <p>This can be any additional information that will be useful when reviewing the report later.</p>
  </>
);

const trace = (
  <>
    <p>Query trace - enable this option to include a query trace in your report.</p>
    <p>This will assist in analyzing and diagnosing the query processing.</p>
  </>
);

const generate = (
  <>
    <p>Generate Report - click this button to generate and save your report. </p>
    <p>After creation, the report can be downloaded and examined on the <Link
      to={router.queryAnalyzer}
      target="_blank"
      rel="noreferrer"
      className="vm-link vm-link_underlined"
    >{routerOptions[router.queryAnalyzer].title}</Link> page.</p>
  </>
);

export default [
  filename,
  comment,
  trace,
  generate,
];
