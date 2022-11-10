import React, { FC, useMemo } from "preact/compat";
import { InstantMetricResult } from "../../../api/types";
import { useSnack } from "../../../contexts/Snackbar";
import { TopQuery } from "../../../types";
import Button from "../../Main/Button/Button";
import "./style.scss";

export interface JsonViewProps {
  data: InstantMetricResult[] | TopQuery[];
}

const JsonView: FC<JsonViewProps> = ({ data }) => {
  const { showInfoMessage } = useSnack();

  const formattedJson = useMemo(() => JSON.stringify(data, null, 2), [data]);

  return (
    <div className="vm-json-view">
      <div className="vm-json-view__copy">
        <Button
          variant="outlined"
          fullWidth={false}
          onClick={(e) => {
            navigator.clipboard.writeText(formattedJson);
            showInfoMessage("Formatted JSON has been copied");
            e.preventDefault(); // needed to avoid snackbar immediate disappearing
          }}
        >
          Copy JSON
        </Button>
      </div>
      <pre className="vm-json-view__code">
        <code>
          {formattedJson}
        </code>
      </pre>
    </div>
  );
};

export default JsonView;
