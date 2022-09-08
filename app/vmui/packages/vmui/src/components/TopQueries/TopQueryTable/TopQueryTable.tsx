import React, {FC, useState, useMemo} from "react";
import TableContainer from "@mui/material/TableContainer";
import Table from "@mui/material/Table";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TableCell from "@mui/material/TableCell";
import TableBody from "@mui/material/TableBody";
import TableSortLabel from "@mui/material/TableSortLabel";
import {TopQuery} from "../../../types";
import {getComparator, stableSort} from "../../Table/helpers";

interface TopQueryTableProps {
  rows: TopQuery[],
}
type ColumnKeys = keyof TopQuery;
const columns: ColumnKeys[] = ["query", "timeRangeSeconds", "avgDurationSeconds", "count", "accountID", "projectID"];

const TopQueryTable:FC<TopQueryTableProps> = ({rows}) => {

  const [orderBy, setOrderBy] = useState("count");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("desc");

  const sortedList = useMemo(() => stableSort(rows as [], getComparator(orderDir, orderBy)),
    [rows, orderBy, orderDir]);

  const onSortHandler = (key: string) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const createSortHandler = (col: string) => () => {
    onSortHandler(col);
  };

  return <TableContainer>
    <Table
      sx={{minWidth: 750}}
      aria-labelledby="tableTitle"
    >
      <TableHead>
        <TableRow>
          {columns.map((col) => (
            <TableCell key={col} sx={{ borderBottomColor: "primary.light" }}>
              <TableSortLabel
                active={orderBy === col}
                direction={orderDir}
                id={col}
                onClick={createSortHandler(col)}
              >
                {col}
              </TableSortLabel>
            </TableCell>
          ))}
        </TableRow>
      </TableHead>
      <TableBody>
        {sortedList.map((row, rowIndex) => (
          <TableRow key={rowIndex}>
            {columns.map((col) => (
              <TableCell
                key={col}
                sx={{
                  borderBottom: rowIndex === rows.length - 1 ? "none" : "",
                  borderBottomColor: "primary.light"
                }}
              >
                {row[col] || "-"}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  </TableContainer>;
};

export default TopQueryTable;
