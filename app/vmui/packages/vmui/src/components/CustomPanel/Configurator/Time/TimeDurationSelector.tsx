import React, {FC} from "preact/compat";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import dayjs from "dayjs";

interface TimeDurationSelector {
  setDuration: (str: string, from: Date) => void;
}

interface DurationOption {
  duration: string,
  title?: string,
  from?: () => Date,
}

const durationOptions: DurationOption[] = [
  {duration: "5m", title: "Last 5 minutes"},
  {duration: "15m", title: "Last 15 minutes"},
  {duration: "30m", title: "Last 30 minutes"},
  {duration: "1h", title: "Last 1 hour"},
  {duration: "3h", title: "Last 3 hours"},
  {duration: "6h", title: "Last 6 hours"},
  {duration: "12h", title: "Last 12 hours"},
  {duration: "24h", title: "Last 24 hours"},
  {duration: "2d", title: "Last 2 days"},
  {duration: "7d", title: "Last 7 days"},
  {duration: "30d", title: "Last 30 days"},
  {duration: "90d", title: "Last 90 days"},
  {duration: "180d", title: "Last 180 days"},
  {duration: "1y", title: "Last 1 year"},
  {duration: "1d", from: () => dayjs().subtract(1, "day").endOf("day").toDate(), title: "Yesterday"},
  {duration: "1d", from: () => dayjs().endOf("day").toDate(), title: "Today"},
];

const TimeDurationSelector: FC<TimeDurationSelector> = ({setDuration}) => {
  // setDurationString("5m"))

  return <List style={{maxHeight: "168px", overflow: "auto", paddingRight: "15px"}}>
    {durationOptions.map(d =>
      <ListItem key={d.duration} button onClick={() => setDuration(d.duration, d.from ? d.from() : new Date())}>
        <ListItemText primary={d.title || d.duration}/>
      </ListItem>)}
  </List>;
};

export default TimeDurationSelector;
