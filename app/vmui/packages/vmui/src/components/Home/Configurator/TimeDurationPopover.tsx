import React, {FC} from "react";
import {Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow} from "@material-ui/core";
import {supportedDurations} from "../../../utils/time";

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