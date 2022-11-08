import React, { FC, useState, useMemo } from "react";
import { TopQuery } from "../../../types";
import { getComparator, stableSort } from "../../../components/Main/Table/helpers";
import { TopQueryPanelProps } from "../TopQueryPanel/TopQueryPanel";

const TopQueryTable:FC<TopQueryPanelProps> = ({ rows, columns, defaultOrderBy }) => {

  const [orderBy, setOrderBy] = useState<keyof TopQuery>(defaultOrderBy || "count");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("desc");

  const sortedList = useMemo(() => stableSort(rows as [], getComparator(orderDir, orderBy)),
    [rows, orderBy, orderDir]);

  const onSortHandler = (key: keyof TopQuery) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const createSortHandler = (col: keyof TopQuery) => () => {
    onSortHandler(col);
  };

  return (
    <div>table</div>
    // <TableContainer>
    //   <Table
    //     sx={{ minWidth: 750 }}
    //     aria-labelledby="tableTitle"
    //   >
    //     <TableHead>
    //       <TableRow>
    //         {columns.map((col) => (
    //           <TableCell
    //             key={col.key}
    //             style={{ width: "100%" }}
    //             sx={{ borderBottomColor: "primary.light", whiteSpace: "nowrap" }}
    //           >
    //             <TableSortLabel
    //               active={orderBy === col.key}
    //               direction={orderDir}
    //               id={col.key}
    //               onClick={createSortHandler(col.key)}
    //             >
    //               {col.title || col.key}
    //             </TableSortLabel>
    //           </TableCell>
    //         ))}
    //       </TableRow>
    //     </TableHead>
    //     <TableBody>
    //       {sortedList.map((row, rowIndex) => (
    //         <TableRow key={rowIndex}>
    //           {columns.map((col) => (
    //             <TableCell
    //               key={col.key}
    //               sx={{
    //                 borderBottom: rowIndex === rows.length - 1 ? "none" : "",
    //                 borderBottomColor: "primary.light"
    //               }}
    //             >
    //               {row[col.key] || "-"}
    //             </TableCell>
    //           ))}
    //         </TableRow>
    //       ))}
    //     </TableBody>
    //   </Table>
    // </TableContainer>
  );
};

export default TopQueryTable;
