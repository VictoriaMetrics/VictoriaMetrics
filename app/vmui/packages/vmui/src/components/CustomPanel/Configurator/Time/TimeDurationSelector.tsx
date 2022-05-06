import React, {FC} from "preact/compat";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import {relativeTimeOptions} from "../../../../utils/time";

interface TimeDurationSelector {
  setDuration: ({duration, until, id}: {duration: string, until: Date, id: string}) => void;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({setDuration}) => {

  return <List style={{maxHeight: "168px", overflow: "auto", paddingRight: "15px"}}>
    {relativeTimeOptions.map(({id, duration, until, title}) =>
      <ListItemButton key={id} onClick={() => setDuration({duration, until: until(), id})}>
        <ListItemText primary={title || duration}/>
      </ListItemButton>)}
  </List>;
};

export default TimeDurationSelector;
