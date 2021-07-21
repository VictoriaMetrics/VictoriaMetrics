import React, {FC, useEffect, useState} from "react";
import {Box, Popover, TextField, Typography} from "@material-ui/core";
import { KeyboardDateTimePicker } from "@material-ui/pickers";
import {TimeDurationPopover} from "./TimeDurationPopover";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import {dateFromSeconds, formatDateForNativeInput} from "../../../utils/time";
import {InlineBtn} from "../../common/InlineBtn";

interface TimeSelectorProps {
  setDuration: (str: string) => void;
  duration: string;
}

export const TimeSelector: FC<TimeSelectorProps> = ({setDuration}) => {

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
      setDuration(durationString);
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

  const open = Boolean(anchorEl);

  return <Box m={1} flexDirection="row" display="flex">
    {/*setup duration*/}
    <Box px={1}>
      <Box>
        <TextField label="Duration" value={durationString} onChange={handleDurationChange}
          fullWidth={true} 
          onBlur={() => {
            setFocused(false);
          }}
          onFocus={() => {
            setFocused(true);
          }}
        />
      </Box>
      <Box my={2}>
        <Typography variant="body2">
          Possible options<span aria-owns={open ? "mouse-over-popover" : undefined}
            aria-haspopup="true"
            style={{cursor: "pointer"}}
            onMouseEnter={handlePopoverOpen}
            onMouseLeave={handlePopoverClose}>�:&nbsp;</span>
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
        <KeyboardDateTimePicker
          variant="inline"
          ampm={false}
          label="Until"
          value={until}
          onChange={date => dispatch({type: "SET_UNTIL", payload: date as unknown as Date})}
          onError={console.log}
          format="DD/MM/YYYY HH:mm:ss"
        />
      </Box>

      <Box my={2}>
        <Typography variant="body2">
          Will be changed to current time for auto-refresh mode.&nbsp;
          <InlineBtn handler={() => dispatch({type: "RUN_QUERY_TO_NOW"})} text="Switch to now"/>
        </Typography>
      </Box>
    </Box>
  </Box>;
};