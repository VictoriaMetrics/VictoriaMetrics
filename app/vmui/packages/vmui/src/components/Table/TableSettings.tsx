import SettingsIcon from "@mui/icons-material/Settings";
import React, {FC, useEffect, useState} from "preact/compat";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Popper from "@mui/material/Popper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import {useSortedCategories} from "../../hooks/useSortedCategories";
import {InstantMetricResult} from "../../api/types";
import FormControl from "@mui/material/FormControl";
import {FormGroup, FormLabel} from "@mui/material";
import FormControlLabel from "@mui/material/FormControlLabel";
import Checkbox from "@mui/material/Checkbox";
import Button from "@mui/material/Button";

const classes = {
  popover: {
    display: "grid",
    gridGap: "16px",
    padding: "0 0 25px",
  },
  popoverHeader: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    background: "#3F51B5",
    padding: "6px 6px 6px 12px",
    borderRadius: "4px 4px 0 0",
    color: "#FFF",
  },
  popoverBody: {
    display: "grid",
    gridGap: "6px",
    padding: "0 14px",
    minWidth: "200px",
  }
};

const title = "Table Settings";

interface TableSettingsProps {
  data: InstantMetricResult[];
  defaultColumns?: string[]
  onChange: (arr: string[]) => void
}

const TableSettings: FC<TableSettingsProps> = ({data, defaultColumns, onChange}) => {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);
  const columns = useSortedCategories(data);
  const [checkedColumns, setCheckedColumns] = useState(columns.map(col => col.key));

  const handleChange = (key: string) => {
    setCheckedColumns(prev => checkedColumns.includes(key) ? prev.filter(col => col !== key) : [...prev, key]);
  };

  const handleClose = () => {
    setAnchorEl(null);
    setCheckedColumns(defaultColumns || columns.map(col => col.key));
  };

  const handleReset = () => {
    setAnchorEl(null);
    const value = columns.map(col => col.key);
    setCheckedColumns(value);
    onChange(value);
  };

  const handleApply = () => {
    setAnchorEl(null);
    onChange(checkedColumns);
  };

  useEffect(() => {
    setCheckedColumns(columns.map(col => col.key));
  }, [columns]);

  return <Box>
    <Tooltip title={title}>
      <IconButton onClick={(e) => setAnchorEl(e.currentTarget)}>
        <SettingsIcon/>
      </IconButton>
    </Tooltip>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="left-start"
      sx={{zIndex: 3}}
      modifiers={[{name: "offset", options: {offset: [0, 6]}}]}>
      <ClickAwayListener onClickAway={() => handleClose()}>
        <Paper elevation={3} sx={classes.popover}>
          <Box id="handle" sx={classes.popoverHeader}>
            <Typography variant="body1"><b>{title}</b></Typography>
            <IconButton size="small" onClick={() => handleClose()}>
              <CloseIcon style={{color: "white"}}/>
            </IconButton>
          </Box>
          <Box sx={classes.popoverBody}>
            <FormControl component="fieldset" variant="standard">
              <FormLabel component="legend">Display columns</FormLabel>
              <FormGroup sx={{display: "grid", maxHeight: "350px", overflow: "auto"}}>
                {columns.map(col => (
                  <FormControlLabel
                    key={col.key}
                    label={col.key}
                    sx={{textTransform: "capitalize"}}
                    control={
                      <Checkbox
                        checked={checkedColumns.includes(col.key)}
                        onChange={() => handleChange(col.key)}
                        name={col.key} />
                    }
                  />
                ))}
              </FormGroup>
            </FormControl>
            <Box display="grid" gridTemplateColumns="1fr 1fr" gap={1} justifyContent="center" mt={2}>
              <Button variant="outlined" onClick={handleReset}>
                Reset
              </Button>
              <Button variant="contained" onClick={handleApply}>
                apply
              </Button>
            </Box>
          </Box>
        </Paper>
      </ClickAwayListener>
    </Popper>
  </Box>;
};

export default TableSettings;
