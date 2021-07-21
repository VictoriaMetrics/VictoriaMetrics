import React, {FC, useMemo} from "react";
import {InstantMetricResult} from "../../../api/types";
import {InstantDataSeries} from "../../../types";
import {Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow} from "@material-ui/core";
import {makeStyles} from "@material-ui/core/styles";
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
      value: d.value[1]
    }));
  }, [sortedColumns, data]);

  return (
    <>
      {(rows.length > 0)
        ? <TableContainer component={Paper}>
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
                <TableRow key={index}>
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