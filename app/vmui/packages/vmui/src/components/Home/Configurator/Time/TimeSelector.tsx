import React, {FC, useEffect, useState} from "preact/compat";
import Box from "@mui/material/Box";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import DateTimePicker from "@mui/lab/DateTimePicker";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {dateFromSeconds, formatDateForNativeInput} from "../../../../utils/time";
import {InlineBtn} from "../../../common/InlineBtn";
import makeStyles from "@mui/styles/makeStyles";
import TimeDurationSelector from "./TimeDurationSelector";
import dayjs from "dayjs";

interface TimeSelectorProps {
  setDuration: (str: string) => void;
}

const useStyles = makeStyles({
  container: {
    display: "grid",
    gridTemplateColumns: "200px 1fr",
    gridGap: "20px",
    height: "100%",
    padding: "20px",
    borderRadius: "4px",
    borderColor: "#b9b9b9",
    borderStyle: "solid",
    borderWidth: "1px"
  },
  timeControls: {
    display: "grid",
    gridTemplateColumns: "1fr",
    gridTemplateRows: "auto 1fr",
    gridGap: "16px 0",
  },
  datePickers: {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit, 200px)",
    gridGap: "16px 0",
  },
  datePickerItem: {
    minWidth: "200px",
  },
});

export const TimeSelector: FC<TimeSelectorProps> = ({setDuration}) => {

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

  return <Box className={classes.container}>
    {/*setup duration*/}
    <Box>
      <TimeDurationSelector setDuration={setDuration}/>
    </Box>
    {/*setup end time*/}
    <Box className={classes.timeControls}>
      <Box className={classes.datePickers}>
        <Box className={classes.datePickerItem}>
          <DateTimePicker
            label="From"
            ampm={false}
            value={from}
            onChange={date => dispatch({type: "SET_FROM", payload: date as unknown as Date})}
            onError={console.log}
            inputFormat="DD/MM/YYYY HH:mm:ss"
            mask="__/__/____ __:__:__"
            renderInput={(params) => <TextField {...params} variant="standard"/>}
            maxDate={dayjs(until)}
          />
        </Box>
        <Box className={classes.datePickerItem}>
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
      </Box>
      <Box>
        <Typography variant="body2">
          Will be changed to current time for auto-refresh mode.&nbsp;
          <InlineBtn handler={() => dispatch({type: "RUN_QUERY_TO_NOW"})} text="Switch to now"/>
        </Typography>
      </Box>
    </Box>
  </Box>;
};
