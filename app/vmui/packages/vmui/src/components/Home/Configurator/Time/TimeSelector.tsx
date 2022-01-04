import React, {FC, useEffect, useState, useMemo} from "preact/compat";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {dateFromSeconds, formatDateForNativeInput} from "../../../../utils/time";
import makeStyles from "@mui/styles/makeStyles";
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

const formatDate = "YYYY-MM-DD HH:mm:ss";

const useStyles = makeStyles({
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
});

export const TimeSelector: FC = () => {

  const classes = useStyles();

  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

  const {time: {period: {end, start}}} = useAppState();
  const dispatch = useAppDispatch();

  useEffect(() => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
  }, [end]);

  useEffect(() => {
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
  }, [start]);

  const setDuration = (dur: string, from: Date) => {
    dispatch({type: "SET_UNTIL", payload: from});
    setAnchorEl(null);
    dispatch({type: "SET_DURATION", payload: dur});
  };

  const formatRange = useMemo(() => {
    const startFormat = dayjs(dateFromSeconds(start)).format(formatDate);
    const endFormat = dayjs(dateFromSeconds(end)).format(formatDate);
    return {
      start: startFormat,
      end: endFormat
    };
  }, [start, end]);

  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

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
        {formatRange.start} - {formatRange.end}
      </Button>
    </Tooltip>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="bottom-end"
      modifiers={[{name: "offset", options: {offset: [0, 6]}}]}>
      <ClickAwayListener onClickAway={() => setAnchorEl(null)}>
        <Paper elevation={3}>
          <Box className={classes.container}>
            <Box className={classes.timeControls}>
              <Box className={classes.datePickerItem}>
                <DateTimePicker
                  label="From"
                  ampm={false}
                  value={from}
                  onChange={date => dispatch({type: "SET_FROM", payload: date as unknown as Date})}
                  onError={console.log}
                  inputFormat={formatDate}
                  mask="____-__-__ __:__:__"
                  renderInput={(params) => <TextField {...params} variant="standard"/>}
                  maxDate={dayjs(until)}
                  PopperProps={{disablePortal: true}}/>
              </Box>
              <Box className={classes.datePickerItem}>
                <DateTimePicker
                  label="To"
                  ampm={false}
                  value={until}
                  onChange={date => dispatch({type: "SET_UNTIL", payload: date as unknown as Date})}
                  onError={console.log}
                  inputFormat={formatDate}
                  mask="____-__-__ __:__:__"
                  renderInput={(params) => <TextField {...params} variant="standard"/>}
                  PopperProps={{disablePortal: true}}/>
              </Box>
              <Box display="grid" gridTemplateColumns="auto 1fr" gap={1}>
                <Button variant="outlined" onClick={() => setAnchorEl(null)}>
                  Cancel
                </Button>
                <Button variant="contained" onClick={() => dispatch({type: "RUN_QUERY_TO_NOW"})}>
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
