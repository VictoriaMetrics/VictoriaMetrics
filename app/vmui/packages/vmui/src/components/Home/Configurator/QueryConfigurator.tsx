import React, {FC, useRef, useState} from "react";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Grid,
  IconButton,
  TextField,
  Typography,
  FormControlLabel,
  Tooltip,
  Switch,
} from "@material-ui/core";
import QueryEditor from "./QueryEditor";
import {TimeSelector} from "./TimeSelector";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import SecurityIcon from "@material-ui/icons/Security";
import {AuthDialog} from "./AuthDialog";
import PlayCircleOutlineIcon from "@material-ui/icons/PlayCircleOutline";
import Portal from "@material-ui/core/Portal";
import {saveToStorage} from "../../../utils/storage";

const QueryConfigurator: FC = () => {

  const {serverUrl, query, time: {duration}} = useAppState();
  const dispatch = useAppDispatch();

  const {queryControls: {autocomplete, nocache}} = useAppState();
  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };
  const onChangeCache = () => {
    dispatch({type: "NO_CACHE"});
    saveToStorage("NO_CACHE", !nocache);
  };

  const [dialogOpen, setDialogOpen] = useState(false);
  const [expanded, setExpanded] = useState(true);

  const queryContainer = useRef<HTMLDivElement>(null);

  const onSetDuration = (dur: string) => dispatch({type: "SET_DURATION", payload: dur});
  const onRunQuery = () => dispatch({type: "RUN_QUERY"});
  const onSetQuery = (query: string) => dispatch({type: "SET_QUERY", payload: query});
  const onSetServer = ({target: {value}}: {target: {value: string}}) => {
    dispatch({type: "SET_SERVER", payload: value});
  };

  return (
    <>
      <Accordion expanded={expanded} onChange={() => setExpanded(prev => !prev)}>
        <AccordionSummary
          expandIcon={<ExpandMoreIcon/>}
          aria-controls="panel1a-content"
          id="panel1a-header"
        >
          <Box mr={2}>
            <Typography variant="h6" component="h2">Query Configuration</Typography>
          </Box>
          <Box flexGrow={1} onClick={e => e.stopPropagation()} onFocusCapture={e => e.stopPropagation()}>
            <Portal disablePortal={!expanded} container={queryContainer.current}>
              <QueryEditor server={serverUrl} query={query} oneLiner={!expanded} autocomplete={autocomplete}
                runQuery={onRunQuery}
                setQuery={onSetQuery}/>
            </Portal>
          </Box>
        </AccordionSummary>
        <AccordionDetails>
          <Grid container spacing={2}>
            <Grid item xs={12} md={6}>
              <Box display="grid" gridGap={16}>
                <Box display="flex" alignItems="center">
                  <TextField variant="outlined" fullWidth label="Server URL" value={serverUrl}
                    inputProps={{
                      style: {fontFamily: "Monospace"}
                    }}
                    onChange={onSetServer}/>
                  <Box ml={1}>
                    <Tooltip title="Execute Query">
                      <IconButton onClick={onRunQuery}>
                        <PlayCircleOutlineIcon />
                      </IconButton>
                    </Tooltip>
                  </Box>
                  <Box>
                    <Tooltip title="Request Auth Settings">
                      <IconButton onClick={() => setDialogOpen(true)}>
                        <SecurityIcon/>
                      </IconButton>
                    </Tooltip>
                  </Box>
                </Box>
                <Box flexGrow={1} >
                  {/* for portal QueryEditor */}
                  <div ref={queryContainer} />
                </Box>
              </Box>
            </Grid>
            <Grid item xs={8} md={6} >
              <Box style={{
                borderRadius: "4px",
                borderColor: "#b9b9b9",
                borderStyle: "solid",
                borderWidth: "1px",
                height: "100%",
              }}>
                <TimeSelector setDuration={onSetDuration} duration={duration}/>
              </Box>
            </Grid>
            <Grid item xs={12}>
              <Box px={1} display="flex" alignItems="center">
                <Box>
                  <FormControlLabel
                    control={<Switch size="small" checked={autocomplete} onChange={onChangeAutocomplete}/>}
                    label="Enable autocomplete"
                  />
                </Box>
                <Box ml={2}>
                  <FormControlLabel
                    control={<Switch size="small" checked={!nocache} onChange={onChangeCache}/>}
                    label="Enable cache"
                  />
                </Box>
              </Box>
            </Grid>
          </Grid>
        </AccordionDetails>
      </Accordion>
      <AuthDialog open={dialogOpen} onClose={() => setDialogOpen(false)}/>
    </>
  );
};

export default QueryConfigurator;