import React, { FC } from "preact/compat";
import { Data, HeadCell } from "../types";
import LineProgress from "../../../../components/Main/LineProgress/LineProgress";
import { PlayCircleOutlineIcon } from "../../../../components/Main/Icons";
import Button from "../../../../components/Main/Button/Button";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";
import classNames from "classnames";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../../constants/date";

interface CardinalityTableCells {
  row: Data;
  tableHeaderCells: HeadCell[];
  totalSeries: number;
  totalSeriesPrev: number;
  onActionClick: (name: string) => void;
}

const TableCells: FC<CardinalityTableCells> = ({
  row,
  tableHeaderCells,
  totalSeries,
  totalSeriesPrev,
  onActionClick,
}) => {
  const progress = totalSeries > 0 ? (row.value / totalSeries) * 100 : -1;
  const progressPrev = totalSeriesPrev > 0 ? (row.valuePrev / totalSeriesPrev) * 100 : -1;
  const hasProgresses = [progress, progressPrev].some(p => p === -1);

  const formattedValue = row.value.toLocaleString("en-US");
  const formattedPrevValue = row.valuePrev.toLocaleString("en-US");
  const formattedDiff = Math.abs(row.diff).toLocaleString("en-US");

  const diffPercentByTotal = progress - progressPrev;
  const relationPrevDay = hasProgresses ? "" : `${Math.abs(diffPercentByTotal).toFixed(2)}%`;

  const diffPercent = `${Math.abs(row.diffPercent).toFixed(2)}%`;

  const neverCount = row.requestsCount === 0;
  const noInfoAboutCount = row.requestsCount == null;
  const formattedCount = noInfoAboutCount
    ? "n/a"
    : row.requestsCount.toLocaleString("en-US");

  const neverRequested = row.lastRequestTimestamp === 0;
  const noInfoAboutRequest = row.lastRequestTimestamp == null;
  const hasLastRequest = !neverRequested && !noInfoAboutRequest;

  const placeholderLastRequest = neverRequested ? "never" : "n/a";
  const lastRequestDiff = hasLastRequest
    ? dayjs().diff(row.lastRequestTimestamp * 1000, "seconds")
    : 0;

  const formattedLastRequestDate = hasLastRequest
    ? dayjs.unix(row.lastRequestTimestamp).format(DATE_TIME_FORMAT)
    : "-";

  const formattedTimeAgo = hasLastRequest
    ? dayjs.duration(-lastRequestDiff, "seconds").humanize(true)
    : placeholderLastRequest;

  const handleActionClick = () => {
    onActionClick(row.name);
  };

  const renderCell = (cell: HeadCell) => {
    switch (cell.id) {
      case "name":
        return (
          <span
            className="vm-link vm-link_colored"
            onClick={handleActionClick}
          >
            {row.name}
          </span>
        );

      case "value":
        return formattedValue;

      case "requestsCount":
        return (
          <div
            className={classNames({
              "vm-dynamic-number": true,
              "vm-dynamic-number_negative": neverCount,
            })}
          >
            {formattedCount}
          </div>
        );

      case "lastRequestTimestamp":
        return (
          <Tooltip title={`${formattedLastRequestDate}`}>
            <span
              className={classNames({
                "vm-dynamic-number": true,
                "vm-dynamic-number_negative": neverRequested || lastRequestDiff >= 30 * 86400, // more than 30 days or never
                "vm-dynamic-number_warning": lastRequestDiff >= 7 * 86400 && lastRequestDiff < 30 * 86400, // 7 - 30 days
              })}
            >
              {formattedTimeAgo}
            </span>
          </Tooltip>
        );

      case "diff":
        return (
          <Tooltip title={`in relation to the previous day: ${formattedPrevValue}`}>
            <span
              className={classNames({
                "vm-dynamic-number": true,
                "vm-dynamic-number_positive vm-dynamic-number_down": row.diff < 0,
                "vm-dynamic-number_negative vm-dynamic-number_up": row.diff > 0,
              })}
            >
              {formattedDiff}
            </span>
          </Tooltip>
        );

      case "diffPercent":
        return (
          <Tooltip title={`in relation to the previous day: ${formattedPrevValue}`}>
            <div
              className={classNames({
                "vm-dynamic-number": true,
                "vm-dynamic-number_positive vm-dynamic-number_down": row.diff < 0,
                "vm-dynamic-number_negative vm-dynamic-number_up": row.diff > 0,
              })}
            >
              {diffPercent}
            </div>
          </Tooltip>
        );

      case "percentage":
        return (
          <div className="vm-cardinality-panel-table__progress">
            <LineProgress
              value={progress}
              hideValue
            />
            <span className="vm-dynamic-number vm-dynamic-number_static">
              {progress.toFixed(2)}%
            </span>
            <Tooltip title="in relation to the previous day">
              <span
                className={classNames({
                  "vm-dynamic-number": true,
                  "vm-dynamic-number_no-change": diffPercentByTotal === 0,
                  "vm-dynamic-number_positive vm-dynamic-number_down": diffPercentByTotal < 0,
                  "vm-dynamic-number_negative vm-dynamic-number_up": diffPercentByTotal > 0,
                })}
              >
                {relationPrevDay}
              </span>
            </Tooltip>
          </div>
        );

      case "action":
        return (
          <div className="vm-table-cell__content">
            <Tooltip title={<span>Filter by <code>`{row.name}`</code></span>}>
              <Button
                variant="text"
                size="small"
                onClick={handleActionClick}
                startIcon={<PlayCircleOutlineIcon />}
              />
            </Tooltip>
          </div>
        );

      default:
        return row?.[cell.id] || "";
    }
  };

  return (
    <>
      {tableHeaderCells.map(cell => (
        <td
          key={cell.id}
          className={classNames(
            "vm-table-cell",
            ...(cell.modifiers?.map(mod => `vm-table-cell_${mod}`) ?? [])
          )}
        >
          {renderCell(cell)}
        </td>
      ))}
    </>
  );
};

export default TableCells;
