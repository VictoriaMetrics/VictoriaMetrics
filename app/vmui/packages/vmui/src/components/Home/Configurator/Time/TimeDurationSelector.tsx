import React, {FC, useEffect, useState} from "react";
import {Box, Popover, TextField, Typography} from "@mui/material";
import {checkDurationLimit} from "../../../../utils/time";
import {TimeDurationPopover} from "./TimeDurationPopover";
import {InlineBtn} from "../../../common/InlineBtn";
import {useAppState} from "../../../../state/common/StateContext";

interface TimeDurationSelector {
  setDuration: (str: string) => void;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({setDuration}) => {
  const {time: {duration}} = useAppState();

  const [anchorEl, setAnchorEl] = React.useState<Element | null>(null);
  const [durationString, setDurationString] = useState<string>(duration);
  const [durationStringFocused, setFocused] = useState(false);
  const open = Boolean(anchorEl);

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

  useEffect(() => {
    setDurationString(duration);
  }, [duration]);

  useEffect(() => {
    if (!durationStringFocused) {
      const value = checkDurationLimit(durationString);
      setDurationString(value);
      setDuration(value);
    }
  }, [durationString, durationStringFocused]);

  return <>
    <Box>
      <TextField label="Duration" value={durationString} onChange={handleDurationChange}
        variant="standard"
        fullWidth={true}
        onKeyUp={onKeyUp}
        onBlur={() => {
          setFocused(false);
        }}
        onFocus={() => {
          setFocused(true);
        }}
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
  </>;
};

export default TimeDurationSelector;