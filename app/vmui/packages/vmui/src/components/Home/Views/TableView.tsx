import React, {FC, useMemo} from "preact/compat";
import {InstantMetricResult} from "../../../api/types";
import {InstantDataSeries} from "../../../types";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import makeStyles from "@mui/styles/makeStyles";
import {useSortedCategories} from "../../../hooks/useSortedCategories";

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

  const rows: InstantDataSeries[] = useMemo(() => {
    return data?.map(d => ({
      metadata: sortedColumns.map(c => d.metric[c.key] || "-"),
      value: d.value ? d.value[1] : "-"
    }));
  }, [sortedColumns, data]);

  return (
    <>
      {(rows.length > 0)
        ? <TableContainer>
          <Table aria-label="simple table">
            <TableHead>
              <TableRow>
                {sortedColumns.map((col, index) => (
                  <TableCell style={{textTransform: "capitalize"}} key={index}>{col.key}</TableCell>))}
                <TableCell align="right">Value</TableCell>
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
        : <div style={{textAlign: "center"}}>No data to show</div>}
    </>
  );
};

export default TableView;