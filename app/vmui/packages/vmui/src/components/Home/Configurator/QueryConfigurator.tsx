import React, {FC, useState} from "react";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Grid,
  IconButton,
  TextField,
  Typography
} from "@material-ui/core";
import QueryEditor from "./QueryEditor";
import {TimeSelector} from "./TimeSelector";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import SecurityIcon from "@material-ui/icons/Security";
import {AuthDialog} from "./AuthDialog";

const QueryConfigurator: FC = () => {

  const {serverUrl, query, time: {duration}} = useAppState();
  const dispatch = useAppDispatch();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [expanded, setExpanded] = useState(true);

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
          {!expanded && <Box flexGrow={1} onClick={e => e.stopPropagation()} onFocusCapture={e => e.stopPropagation()}>
            <QueryEditor server={serverUrl} query={query} oneLiner setQuery={(query) => dispatch({type: "SET_QUERY", payload: query})}/>
          </Box>}
        </AccordionSummary>
        <AccordionDetails>
          <Grid container spacing={2}>
            <Grid item xs={12} md={6}>
              <Box>
                <Box py={2} display="flex">
                  <TextField variant="outlined" fullWidth label="Server URL" value={serverUrl}
                    inputProps={{
                      style: {fontFamily: "Monospace"}
                    }}
                    onChange={(e) => dispatch({type: "SET_SERVER", payload: e.target.value})}/>
                  <Box pl={.5} flexGrow={0}>
                    <IconButton onClick={() => setDialogOpen(true)}>
                      <SecurityIcon/>
                    </IconButton>
                  </Box>
                </Box>
                <QueryEditor server={serverUrl} query={query} setQuery={(query) => dispatch({type: "SET_QUERY", payload: query})}/>

              </Box>
            </Grid>
            <Grid item xs={12} md={6}>
              <Box style={{
                borderRadius: "4px",
                borderColor: "#b9b9b9",
                borderStyle: "solid",
                borderWidth: "1px",
                height: "calc(100% - 18px)",
                marginTop: "16px"
              }}>
                <TimeSelector setDuration={(dur) => dispatch({type: "SET_DURATION", payload: dur})} duration={duration}/>
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