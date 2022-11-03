import React, {FC, useState} from "preact/compat";
import {ChangeEvent} from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import {Alert} from "@mui/material";
import Trace from "../../components/CustomPanel/Trace/Trace";
import TracingsView from "../../components/CustomPanel/Views/TracingsView";
import Tooltip from "@mui/material/Tooltip";

const TracePage: FC = () => {
  const [tracesState, setTracesState] = useState<Trace[]>([]);
  const [errors, setErrors] = useState<string[]>([]);

  const handleError = (e: Error, filename?: string) => {
    setErrors(prev => [...prev, `[${filename}] ${e.name}: ${e.message}`]);
  };

  const handleOnload = (e: ProgressEvent<FileReader>, filename: string) => {
    try {
      const result = String(e.target?.result);
      const resp = JSON.parse(result);
      const traceData = resp.trace || resp;
      if (!traceData.duration_msec) {
        handleError(new Error("Not found the tracing information"), filename);
        return;
      }
      setTracesState(prev => [...prev, new Trace(traceData, filename)]);
    } catch (e) {
      if (e instanceof Error) handleError(e, filename);
    }
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setErrors([]);
    const files = Array.from(e.target.files || []);
    files.map(f => {
      const reader = new FileReader();
      const filename = f?.name || "";
      reader.onload = (e) => handleOnload(e, filename);
      reader.readAsText(f);
    });
    e.target.value = "";
  };

  const handleTraceDelete = (trace: Trace) => {
    const updatedTraces = tracesState.filter((data) => data.idValue !== trace.idValue);
    setTracesState([...updatedTraces]);
  };

  return (
    <Box p={4} display={"flex"} flexDirection={"column"} minHeight={"calc(100vh - 64px)"}>
      <Box display="grid" gridTemplateColumns="1fr auto" alignItems="start" gap={4} mb={4}>
        {!!errors.length && (
          <Alert
            color="error"
            severity="error"
            sx={{width: "100%", whiteSpace: "pre-wrap"}}
          >
            {errors.map((error, i) => <Box mb={1} key={`${error}_${i}`}>{error}</Box>)}
          </Alert>
        )}
        <Box
          gridColumn={2}
          display="flex"
          gap={1}
          flexDirection="column"
          justifyContent="flex-end"
          alignItems="flex-end"
          justifySelf="center"
        >
          <Tooltip title="The file must contain tracing information in JSON format">
            <Button
              variant="contained"
              component="label"
            >
              Upload File
              <input
                type="file"
                hidden
                accept="application/json"
                multiple
                onChange={handleChange}
              />
            </Button>
          </Tooltip>
        </Box>
      </Box>

      {!!tracesState.length && (
        <Box>
          <TracingsView traces={tracesState} onDeleteClick={handleTraceDelete}/>
        </Box>
      )}

      {!tracesState.length && (
        <Box
          display={"flex"}
          gap={1}
          flexDirection={"column"}
          alignItems={"center"}
          justifyContent={"center"}
          flexGrow={"1"}
          paddingBottom={"64px"}
        >
          <div>Please, file with JSON response content.</div>
          <div>The file must contain tracing information in JSON format.</div>
          <div>Tracing graph will be displayed after file upload.</div>
          <Tooltip title="The file must contain tracing information in JSON format">
            <Button
              variant="contained"
              component="label"
              size={"large"}
              sx={{mt: 2}}
            >
              Upload File
              <input
                type="file"
                hidden
                accept="application/json"
                multiple
                onChange={handleChange}
              />
            </Button>
          </Tooltip>
        </Box>
      )}
    </Box>
  );
};

export default TracePage;
