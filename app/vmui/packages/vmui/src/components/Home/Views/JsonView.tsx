import React, {FC, useMemo} from "preact/compat";
import {InstantMetricResult} from "../../../api/types";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import {useSnack} from "../../../contexts/Snackbar";

export interface JsonViewProps {
  data: InstantMetricResult[];
}

const JsonView: FC<JsonViewProps> = ({data}) => {
  const {showInfoMessage} = useSnack();

  const formattedJson = useMemo(() => JSON.stringify(data, null, 2), [data]);

  return (
    <Box position="relative">
      <Box
        style={{
          position: "sticky",
          top: "16px",
          display: "flex",
          justifyContent: "flex-end",
        }}>
        <Button variant="outlined"
          fullWidth={false}
          onClick={(e) => {
            navigator.clipboard.writeText(formattedJson);
            showInfoMessage("Formatted JSON has been copied");
            e.preventDefault(); // needed to avoid snackbar immediate disappearing
          }}>
          Copy JSON
        </Button>
      </Box>
      <pre style={{margin: 0}}>{formattedJson}</pre>
    </Box>
  );
};

export default JsonView;
