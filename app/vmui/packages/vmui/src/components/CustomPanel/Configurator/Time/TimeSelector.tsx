import React, {FC, useEffect, useState, useMemo} from "preact/compat";
import {KeyboardEvent} from "react";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {dateFromSeconds, formatDateForNativeInput} from "../../../../utils/time";
import TimeDurationSelector from "./TimeDurationSelector";
import dayjs from "dayjs";
import QueryBuilderIcon from "@mui/icons-material/QueryBuilder";
import Box from "@mui/material/Box";
import TextField from "@mui/material/TextField";
import DateTimePicker from "@mui/lab/DateTimePicker";
import Button from "@mui/material/Button";
import Popper from "@mui/material/Popper";
import Paper from "@mui/material/Paper";
import Divider from "@mui/material/Divider";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import Tooltip from "@mui/material/Tooltip";
import AlarmAdd from "@mui/icons-material/AlarmAdd";

const formatDate = "YYYY-MM-DD HH:mm:ss";

const classes = {
  container: {
    display: "grid",
    gridTemplateColumns: "200px auto 200px",
    gridGap: "10px",
    padding: "20px",
  },
  timeControls: {
    display: "grid",
    gridTemplateRows: "auto 1fr auto",
    gridGap: "16px 0",
  },
  datePickerItem: {
    minWidth: "200px",
  },
};

export const TimeSelector: FC = () => {

  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

  const {time: {period: {end, start}, relativeTime}} = useAppState();
  const dispatch = useAppDispatch();

  useEffect(() => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
  }, [end]);

  useEffect(() => {
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
  }, [start]);

  const setDuration = ({duration, until, id}: {duration: string, until: Date, id: string}) => {
    dispatch({type: "SET_RELATIVE_TIME", payload: {duration, until, id}});
    setAnchorEl(null);
  };

  const formatRange = useMemo(() => {
    const startFormat = dayjs(dateFromSeconds(start)).format(formatDate);
    const endFormat = dayjs(dateFromSeconds(end)).format(formatDate);
    return {
      start: startFormat,
      end: endFormat
    };
  }, [start, end]);

  const open = Boolean(anchorEl);
  const setTimeAndClosePicker = () => {
    if (from) {
      dispatch({type: "SET_FROM", payload: new Date(from)});
    }
    if (until) {
      dispatch({type: "SET_UNTIL", payload: new Date(until)});
    }
    setAnchorEl(null);
  };
  const onFromChange = (from: dayjs.Dayjs | null) => setFrom(from?.format(formatDate));
  const onUntilChange = (until: dayjs.Dayjs | null) => setUntil(until?.format(formatDate));
  const onApplyClick = () => setTimeAndClosePicker();
  const onSwitchToNow = () => dispatch({type: "RUN_QUERY_TO_NOW"});
  const onCancelClick = () => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
    setAnchorEl(null);
  };
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter" || e.keyCode === 13) {
      setTimeAndClosePicker();
    }
  };

  return <>
    <Tooltip title="Time range controls">
      <Button variant="contained" color="primary"
        sx={{
          color: "white",
          border: "1px solid rgba(0, 0, 0, 0.2)",
          boxShadow: "none"
        }}
        startIcon={<QueryBuilderIcon/>}
        onClick={(e) => setAnchorEl(e.currentTarget)}>
        {relativeTime && relativeTime !== "none"
          ? relativeTime.replace(/_/g, " ")
          : `${formatRange.start} - ${formatRange.end}`}
      </Button>
    </Tooltip>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="bottom-end"
      modifiers={[{name: "offset", options: {offset: [0, 6]}}]}>
      <ClickAwayListener onClickAway={() => setAnchorEl(null)}>
        <Paper elevation={3}>
          <Box sx={classes.container}>
            <Box sx={classes.timeControls}>
              <Box sx={classes.datePickerItem}>
                <DateTimePicker
                  label="From"
                  ampm={false}
                  value={from}
                  onChange={onFromChange}
                  onError={console.log}
                  inputFormat={formatDate}
                  mask="____-__-__ __:__:__"
                  renderInput={(params) => <TextField {...params} variant="standard" onKeyDown={onKeyDown}/>}
                  maxDate={dayjs(until)}
                  PopperProps={{disablePortal: true}}/>
              </Box>
              <Box sx={classes.datePickerItem}>
                <DateTimePicker
                  label="To"
                  ampm={false}
                  value={until}
                  onChange={onUntilChange}
                  onError={console.log}
                  inputFormat={formatDate}
                  mask="____-__-__ __:__:__"
                  renderInput={(params) => <TextField {...params} variant="standard" onKeyDown={onKeyDown}/>}
                  PopperProps={{disablePortal: true}}/>
              </Box>
              <Box display="grid" gridTemplateColumns="auto 1fr" gap={1}>
                <Button variant="outlined" onClick={onCancelClick}>
                  Cancel
                </Button>
                <Button variant="outlined" onClick={onApplyClick} color={"success"}>
                  Apply
                </Button>
                <Button startIcon={<AlarmAdd />} onClick={onSwitchToNow}>
                  switch to now
                </Button>
              </Box>
            </Box>
            {/*setup duration*/}
            <Divider orientation="vertical" flexItem />
            <Box>
              <TimeDurationSelector setDuration={setDuration}/>
            </Box>
          </Box>
        </Paper>
      </ClickAwayListener>
    </Popper>
  </>;
};
