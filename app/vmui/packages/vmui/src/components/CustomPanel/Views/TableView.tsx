import React, {FC, useMemo, useState} from "preact/compat";
import {InstantMetricResult} from "../../../api/types";
import {InstantDataSeries} from "../../../types";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TableSortLabel from "@mui/material/TableSortLabel";
import makeStyles from "@mui/styles/makeStyles";
import {useSortedCategories} from "../../../hooks/useSortedCategories";
import Alert from "@mui/material/Alert";

export interface GraphViewProps {
  data: InstantMetricResult[];
}

const useStyles = makeStyles({
  deemphasized: {
    opacity: 0.4
  }
});

const TableView: FC<GraphViewProps> = ({data}) => {

  const classes = useStyles();

  const sortedColumns = useSortedCategories(data);

  const [orderBy, setOrderBy] = useState("");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("asc");

  const rows: InstantDataSeries[] = useMemo(() => {
    const rows = data?.map(d => ({
      metadata: sortedColumns.map(c => d.metric[c.key] || "-"),
      value: d.value ? d.value[1] : "-"
    }));
    const orderByValue = orderBy === "Value";
    const rowIndex = sortedColumns.findIndex(c => c.key === orderBy);
    if (!orderByValue && rowIndex === -1) return rows;
    return rows.sort((a,b) => {
      const n1 = orderByValue ? Number(a.value) : a.metadata[rowIndex];
      const n2 = orderByValue ? Number(b.value) : b.metadata[rowIndex];
      const asc = orderDir === "asc" ? n1 < n2 : n1 > n2;
      return asc ? -1 : 1;
    });
  }, [sortedColumns, data, orderBy, orderDir]);

  const sortHandler = (key: string) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };


  return (
    <>
      {(rows.length > 0)
        ? <TableContainer>
          <Table aria-label="simple table">
            <TableHead>
              <TableRow>
                {sortedColumns.map((col, index) => (
                  <TableCell key={index} style={{textTransform: "capitalize"}}>
                    <TableSortLabel
                      active={orderBy === col.key}
                      direction={orderDir}
                      onClick={() => sortHandler(col.key)}
                    >
                      {col.key}
                    </TableSortLabel>
                  </TableCell>
                ))}
                <TableCell align="right">
                  <TableSortLabel
                    active={orderBy === "Value"}
                    direction={orderDir}
                    onClick={() => sortHandler("Value")}
                  >
                    Value
                  </TableSortLabel>
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((row, index) => (
                <TableRow key={index} hover>
                  {row.metadata.map((rowMeta, index2) => {
                    const prevRowValue = rows[index - 1] && rows[index - 1].metadata[index2];
                    return (
                      <TableCell className={prevRowValue === rowMeta ? classes.deemphasized : undefined}
                        key={index2}>{rowMeta}</TableCell>
                    );
                  }
                  )}
                  <TableCell align="right">{row.value}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
        : <Alert color="warning" severity="warning" sx={{mt: 2}}>No data to show</Alert>}
    </>
  );
};

export default TableView;