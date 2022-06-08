import {MouseEvent} from "react";
import {Box, TableCell, TableHead, TableRow, TableSortLabel} from "@mui/material";
import {visuallyHidden} from "@mui/utils";
import React from "preact/compat";
import {Data, EnhancedHeaderTableProps} from "./types";


export function EnhancedTableHead(props: EnhancedHeaderTableProps) {
  const { order, orderBy, onRequestSort, headerCells } = props;
  const createSortHandler =
    (property: keyof Data) => (event: MouseEvent<unknown>) => {
      onRequestSort(event, property);
    };

  return (
    <TableHead>
      <TableRow>
        {headerCells.map((headCell) => (
          <TableCell
            key={headCell.id}
            align={headCell.numeric ? "right" : "left"}
            sortDirection={orderBy === headCell.id ? order : false}
          >
            <TableSortLabel
              active={orderBy === headCell.id}
              direction={orderBy === headCell.id ? order : "asc"}
              onClick={createSortHandler(headCell.id as keyof Data)}
            >
              {headCell.label}
              {orderBy === headCell.id ? (
                <Box component="span" sx={visuallyHidden}>
                  {order === "desc" ? "sorted descending" : "sorted ascending"}
                </Box>
              ) : null}
            </TableSortLabel>
          </TableCell>
        ))}
      </TableRow>
    </TableHead>
  );
}
