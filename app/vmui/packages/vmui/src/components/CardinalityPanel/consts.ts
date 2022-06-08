import {HeadCell} from "../Table/types";

export const headCellsWithProgress = [
  {
    disablePadding: false,
    id: "name",
    label: "Name",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Value",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "percentage",
    label: "Percent of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];

export const defaultHeadCells = headCellsWithProgress.filter((head) => head.id!=="percentage");

export const spinnerContainerStyles = (height: string) =>  {
  return {
    width: "100%",
    maxWidth: "100%",
    position: "absolute",
    height: height ?? "50%",
    background: "rgba(255, 255, 255, 0.7)",
    pointerEvents: "none",
    zIndex: 1000,
  };
};

export const SPINNER_TITLE = "Please wait while cardinality stats is calculated. This may take some time if the db contains big number of time series";
