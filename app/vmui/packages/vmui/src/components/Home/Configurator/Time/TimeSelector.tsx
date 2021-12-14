import React, {FC, useEffect, useState} from "react";
import {Box, Popover, TextField, Typography} from "@mui/material";
import DateTimePicker from "@mui/lab/DateTimePicker";
import {TimeDurationPopover} from "./TimeDurationPopover";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {checkDurationLimit, dateFromSeconds, formatDateForNativeInput} from "../../../../utils/time";
import {InlineBtn} from "../../../common/InlineBtn";
import makeStyles from "@mui/styles/makeStyles";

interface TimeSelectorProps {
  setDuration: (str: string) => void;
  duration: string;
}

const useStyles = makeStyles({
  container: {
    display: "grid",
    gridTemplateColumns: "auto auto",
    height: "100%",
    padding: "18px 14px",
    borderRadius: "4px",
    borderColor: "#b9b9b9",
    borderStyle: "solid",
    borderWidth: "1px"
  }
});

export const TimeSelector: FC<TimeSelectorProps> = ({setDuration}) => {

  const classes = useStyles();

  const [durationStringFocused, setFocused] = useState(false);
  const [anchorEl, setAnchorEl] = React.useState<Element | null>(null);
  const [until, setUntil] = useState<string>();

  const {time: {period: {end}, duration}} = useAppState();

  const dispatch = useAppDispatch();

  const [durationString, setDurationString] = useState<string>(duration);

  useEffect(() => {
    setDurationString(duration);
  }, [duration]);

  useEffect(() => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
  }, [end]);

  useEffect(() => {
    if (!durationStringFocused) {
      const value = checkDurationLimit(durationString);
      setDurationString(value);
      setDuration(value);
    }
  }, [durationString, durationStringFocused]);

  const handleDurationChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setDurationString(event.target.value);
  };

  const handlePopoverOpen = (event: React.MouseEvent<Element, MouseEvent>) => {
    setAnchorEl(event.currentTarget);
  };

  const handlePopoverClose = () => {
    setAnchorEl(null);
  };

  const onKeyUp = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter") return;
    const target = event.target as HTMLInputElement;
    target.blur();
    setDurationString(target.value);
  };

  const open = Boolean(anchorEl);

  return <Box className={classes.container}>
    {/*setup duration*/}
    <Box px={1}>
      <Box>
        <TextField label="Duration" value={durationString} onChange={handleDurationChange}
          variant="standard"
          fullWidth={true}
          onKeyUp={onKeyUp}
          onBlur={() => {setFocused(false);}}
          onFocus={() => {setFocused(true);}}
        />
      </Box>
      <Box mt={2}>
        <Typography variant="body2">
          <span aria-owns={open ? "mouse-over-popover" : undefined}
            aria-haspopup="true"
            style={{cursor: "pointer"}}
            onMouseEnter={handlePopoverOpen}
            onMouseLeave={handlePopoverClose}>
            Possible options:&nbsp;
          </span>
          <Popover
            open={open}
            anchorEl={anchorEl}
            anchorOrigin={{
              vertical: "bottom",
              horizontal: "left",
            }}
            transformOrigin={{
              vertical: "top",
              horizontal: "left",
            }}
            style={{pointerEvents: "none"}} // important
            onClose={handlePopoverClose}
            disableRestoreFocus
          >
            <TimeDurationPopover/>
          </Popover>
          <InlineBtn handler={() => setDurationString("5m")} text="5m"/>,&nbsp;
          <InlineBtn handler={() => setDurationString("1h")} text="1h"/>,&nbsp;
          <InlineBtn handler={() => setDurationString("1h 30m")} text="1h 30m"/>
        </Typography>
      </Box>
    </Box>
    {/*setup end time*/}
    <Box px={1}>
      <Box>
        <DateTimePicker
          label="Until"
          ampm={false}
          value={until}
          onChange={date => dispatch({type: "SET_UNTIL", payload: date as unknown as Date})}
          onError={console.log}
          inputFormat="DD/MM/YYYY HH:mm:ss"
          mask="__/__/____ __:__:__"
          renderInput={(params) => <TextField {...params} variant="standard"/>}
        />
      </Box>

      <Box mt={2}>
        <Typography variant="body2">
          Will be changed to current time for auto-refresh mode.&nbsp;
          <InlineBtn handler={() => dispatch({type: "RUN_QUERY_TO_NOW"})} text="Switch to now"/>
        </Typography>
      </Box>
    </Box>
  </Box>;
};
