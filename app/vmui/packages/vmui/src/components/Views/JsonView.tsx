import React, { FC, useMemo } from "preact/compat";
import { InstantMetricResult } from "../../api/types";
import { useSnack } from "../../contexts/Snackbar";
import { TopQuery } from "../../types";
import Button from "../Main/Button/Button";

export interface JsonViewProps {
  data: InstantMetricResult[] | TopQuery[];
}

const JsonView: FC<JsonViewProps> = ({ data }) => {
  const { showInfoMessage } = useSnack();

  const formattedJson = useMemo(() => JSON.stringify(data, null, 2), [data]);

  return (
    <div>
      <div
        style={{
          position: "sticky",
          top: "16px",
          display: "flex",
          justifyContent: "flex-end",
        }}
      >
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
      <pre style={{ margin: 0 }}>{formattedJson}</pre>
    </div>
  );
};

export default JsonView;
