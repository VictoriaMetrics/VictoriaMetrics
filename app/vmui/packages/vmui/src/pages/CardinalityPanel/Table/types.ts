import { MouseEvent, ReactNode } from "react";

export type Order = "asc" | "desc";

export interface HeadCell {
  id: string;
  label: string | ReactNode;
  info?: string;
}

export interface EnhancedHeaderTableProps {
  onRequestSort: (event: MouseEvent<unknown>, property: keyof Data) => void;
  order: Order;
  orderBy: string;
  rowCount: number;
  headerCells: HeadCell[];
}

export interface TableProps {
  rows: Data[];
  headerCells: HeadCell[],
  defaultSortColumn: keyof Data,
  tableCells: (row: Data) => ReactNode,
  isPagingEnabled?: boolean,
}


export interface Data {
  name: string;
  value: number;
  diff: number;
  valuePrev: number;
  progressValue: number;
  actions: string;
}
