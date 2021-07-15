import React, {FC, useEffect, useState} from "react";
import {Box, FormControlLabel, IconButton, Switch, Tooltip} from "@material-ui/core";
import PlayCircleOutlineIcon from "@material-ui/icons/PlayCircleOutline";

import EqualizerIcon from "@material-ui/icons/Equalizer";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import CircularProgressWithLabel from "../../common/CircularProgressWithLabel";
import {makeStyles} from "@material-ui/core/styles";

const useStyles = makeStyles({
  colorizing: {
    color: "white"
  }
});

export const ExecutionControls: FC = () => {
  const classes = useStyles();

  const dispatch = useAppDispatch();
  const {queryControls: {autoRefresh}} = useAppState();

  const [delay, setDelay] = useState<(1|2|5)>(5);
  const [lastUpdate, setLastUpdate] = useState<number|undefined>();
  const [progress, setProgress] = React.useState(100);

  const handleChange = () => {
    dispatch({type: "TOGGLE_AUTOREFRESH"});
  };

  useEffect(() => {
    let timer: number;
    if (autoRefresh) {
      setLastUpdate(new Date().valueOf());
      timer = setInterval(() => {
        setLastUpdate(new Date().valueOf());
        dispatch({type: "RUN_QUERY_TO_NOW"});
      }, delay * 1000) as unknown as number;
    }
    return () => {
      timer && clearInterval(timer);
    };
  }, [delay, autoRefresh]);

  useEffect(() => {
    const timer = setInterval(() => {
      if (autoRefresh && lastUpdate) {
        const delta = (new Date().valueOf() - lastUpdate) / 1000; //s
        const nextValue = Math.floor(delta / delay * 100);
        setProgress(nextValue);
      }
    }, 16);
    return () => {
      clearInterval(timer);
    };
  }, [autoRefresh, lastUpdate, delay]);

  const iterateDelays = () => {
    setDelay(prev => {
      switch (prev) {
        case 1:
          return 2;
        case 2:
          return 5;
        case 5:
          return 1;
        default:
          return 5;
      }
    });
  };

  return <Box display="flex" alignItems="center">
    <Box mr={2}>
      <Tooltip title="Execute Query">
        <IconButton onClick={()=>dispatch({type: "RUN_QUERY"})}>
          <PlayCircleOutlineIcon className={classes.colorizing} fontSize="large"/>
        </IconButton>
      </Tooltip>
    </Box>
    {<FormControlLabel
      control={<Switch size="small" className={classes.colorizing} checked={autoRefresh} onChange={handleChange} />}
      label="Auto-refresh"
    />}

    {autoRefresh && <>
      <CircularProgressWithLabel className={classes.colorizing} label={delay} value={progress} onClick={() => {iterateDelays();}} />
      <Box ml={1}>
        <IconButton onClick={() => {iterateDelays();}}><EqualizerIcon style={{color: "white"}} /></IconButton>
      </Box>
    </>}
  </Box>;
};