import { Order } from "../../pages/CardinalityPanel/Table/types";
import dayjs from "dayjs";

const dateColumns = ["date", "timestamp", "time"];

export function descendingComparator<T>(a: T, b: T, orderBy: keyof T) {
  const valueA = a[orderBy];
  const valueB = b[orderBy];
  const parsedValueA = dateColumns.includes(`${orderBy}`) ? dayjs(`${valueA}`).unix() : valueA;
  const parsedValueB = dateColumns.includes(`${orderBy}`) ? dayjs(`${valueB}`).unix() : valueB;
  if (parsedValueB < parsedValueA) {
    return -1;
  }
  if (parsedValueB > parsedValueA) {
    return 1;
  }
  return 0;
}

export function getComparator<Key extends (string | number | symbol)>(
  order: Order,
  orderBy: Key,
): (
  a: { [key in Key]: number | string },
  b: { [key in Key]: number | string },
) => number {
  return order === "desc"
    ? (a, b) => descendingComparator(a, b, orderBy)
    : (a, b) => -descendingComparator(a, b, orderBy);
}

// This method is created for cross-browser compatibility, if you don't
// need to support IE11, you can use Array.prototype.sort() directly
export function stableSort<T>(array: readonly T[], comparator: (a: T, b: T) => number) {
  const stabilizedThis = array.map((el, index) => [el, index] as [T, number]);
  stabilizedThis.sort((a, b) => {
    const order = comparator(a[0], b[0]);
    if (order !== 0) {
      return order;
    }
    return a[1] - b[1];
  });
  return stabilizedThis.map((el) => el[0]);
}
