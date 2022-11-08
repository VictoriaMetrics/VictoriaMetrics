import React, { FC } from "react";
import { useState } from "preact/compat";
import dayjs from "dayjs";
import { getAppModeEnable } from "../../../utils/app-mode";

const formatDate = "YYYY-MM-DD";

interface DatePickerProps {
  date: string | null,
  onChange: (val: string | null) => void
}

const DatePicker: FC<DatePickerProps> = ({ date, onChange }) => {

  const appModeEnable = getAppModeEnable();
  const dateFormatted = date ? dayjs(date).format(formatDate) : null;

  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

  return <>
    {/*<Tooltip title="Date control">*/}
    {/*  <Button*/}
    {/*    variant="contained"*/}
    {/*    color="primary"*/}
    {/*    sx={{*/}
    {/*      color: "white",*/}
    {/*      border: appModeEnable ? "none" : "1px solid rgba(0, 0, 0, 0.2)",*/}
    {/*      boxShadow: "none"*/}
    {/*    }}*/}
    {/*    startIcon={<EventIcon/>}*/}
    {/*    onClick={(e) => setAnchorEl(e.currentTarget)}*/}
    {/*  >*/}
    {/*    {dateFormatted}*/}
    {/*  </Button>*/}
    {/*</Tooltip>*/}
    {/*<Popper*/}
    {/*  open={open}*/}
    {/*  anchorEl={anchorEl}*/}
    {/*  placement="bottom-end"*/}
    {/*  modifiers={[{ name: "offset", options: { offset: [0, 6] } }]}*/}
    {/*>*/}
    {/*  <ClickAwayListener onClickAway={() => setAnchorEl(null)}>*/}
    {/*    <Paper elevation={3}>*/}
    {/*      <Box>*/}
    {/*        <StaticDatePicker*/}
    {/*          displayStaticWrapperAs="desktop"*/}
    {/*          inputFormat={formatDate}*/}
    {/*          mask="____-__-__"*/}
    {/*          value={date}*/}
    {/*          onChange={(newDate) => {*/}
    {/*            onChange(newDate ? dayjs(newDate).format(formatDate) : null);*/}
    {/*            setAnchorEl(null);*/}
    {/*          }}*/}
    {/*          renderInput={(params) => <TextField {...params}/>}*/}
    {/*        />*/}
    {/*      </Box>*/}
    {/*    </Paper>*/}
    {/*  </ClickAwayListener>*/}
    {/*</Popper>*/}
    <div>coming soon</div>
  </>;
};

export default DatePicker;
