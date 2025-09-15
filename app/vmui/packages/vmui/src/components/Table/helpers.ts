import { Order } from "../../pages/CardinalityPanel/Table/types";
import { getNanoTimestamp } from "../../utils/time";

const dateColumns = ["date", "timestamp", "time"];

export function descendingComparator<T>(a: T, b: T, orderBy: keyof T) {
  const valueA = a[orderBy];
  const valueB = b[orderBy];

  // null/undefined
  if (valueA == null && valueB == null) return 0;
  if (valueA == null) return 1;
  if (valueB == null) return -1;

  const strA = String(valueA);
  const strB = String(valueB);

  // Dates
  const isDate = dateColumns.includes(String(orderBy));
  if (isDate) {
    const timeA = getNanoTimestamp(strA);
    const timeB = getNanoTimestamp(strB);

    if (timeB < timeA) return -1;
    if (timeB > timeA) return 1;
    return 0;
  }

  // Numbers
  const numA = Number(strA);
  const numB = Number(strB);
  const isNumeric = !isNaN(numA) && !isNaN(numB);

  if (isNumeric) {
    return numB - numA;
  }

  // Strings
  if (strB < strA) return -1;
  if (strB > strA) return 1;
  return 0;
}

export function getComparator<T extends object>(
  order: Order,
  orderBy: keyof T,
): (
  a: T,
  b: T,
) => number {
  return order === "desc"
    ? (a, b) => descendingComparator(a, b, orderBy)
    : (a, b) => -descendingComparator(a, b, orderBy);
}

// This method is created for cross-browser compatibility, if you don't
// need to support IE11, you can use Array.prototype.sort() directly
export function stableSort<T>(array: readonly T[], comparator: (a: T, b: T) => number): T[] {
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
