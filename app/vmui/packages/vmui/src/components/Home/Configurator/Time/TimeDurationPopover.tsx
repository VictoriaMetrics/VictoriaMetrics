import React, {FC} from "preact/compat";
import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import {supportedDurations} from "../../../../utils/time";

export const TimeDurationPopover: FC = () => {

  return <TableContainer component={Paper}>
    <Table aria-label="simple table" size="small">
      <TableHead>
        <TableRow>
          <TableCell>Long</TableCell>
          <TableCell>Short</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {supportedDurations.map((row, index) => (
          <TableRow key={index}>
            <TableCell component="th" scope="row">{row.long}</TableCell>
            <TableCell>{row.short}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  </TableContainer>;
};