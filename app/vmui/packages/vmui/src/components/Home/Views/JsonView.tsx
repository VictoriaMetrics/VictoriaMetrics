import React, {FC, useMemo} from "react";
import {InstantMetricResult} from "../../../api/types";
import {Box, Button} from "@material-ui/core";
import {useSnack} from "../../../contexts/Snackbar";

export interface JsonViewProps {
  data: InstantMetricResult[];
}

const JsonView: FC<JsonViewProps> = ({data}) => {
  const {showInfoMessage} = useSnack();

  const formattedJson = useMemo(() => JSON.stringify(data, null, 2), [data]);

  return (
    <Box position="relative">
      <Box flexDirection="column" justifyContent="flex-end" display="flex"
        style={{
          position: "fixed",
          right: "16px"
        }}>
        <Button variant="outlined" onClick={(e) => {
          navigator.clipboard.writeText(formattedJson);
          showInfoMessage("Formatted JSON has been copied");
          e.preventDefault(); // needed to avoid snackbar immediate disappearing
        }}>Copy JSON</Button>
      </Box>
      <pre>{formattedJson}</pre>
    </Box>
  );
};

export default JsonView;
