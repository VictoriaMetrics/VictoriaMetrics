import React, { FC } from "preact/compat";
import { InfoOutlinedIcon } from "../../../components/Main/Icons";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { MetricNameStats } from "../types";
import "./style.scss";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";

interface Props {
  metricNameStats: MetricNameStats;
}

const CardinalityMetricNameStats: FC<Props> = ({ metricNameStats }) => {
  const {
    statsCollectedSince,
    statsCollectedRecordsTotal = 0,
    trackerMemoryMaxSizeBytes = 0,
    trackerCurrentMemoryUsageBytes = 0
  } = metricNameStats;

  const date = statsCollectedSince ? dayjs.unix(statsCollectedSince).format(DATE_TIME_FORMAT) : " - ";
  const cache = trackerMemoryMaxSizeBytes ? `${(trackerCurrentMemoryUsageBytes / trackerMemoryMaxSizeBytes * 100).toFixed(2)}%` : " - ";
  const total = statsCollectedRecordsTotal.toLocaleString("en-US");


  return (
    <div className="vm-cardinality-totals-card">
      <h4 className="vm-cardinality-totals-card__title">
        <Tooltip
          placement="bottom-left"
          title={
            <div className="vm-cardinality-totals-card__tooltip">
              {statsCollectedSince ? (
                <>
                  <p>Total entries in cache since {date}</p>
                  <p>Cache utilization: {cache}</p>
                </>
              ) : (
                <>
                  <p>Metric names tracker is likely disabled.</p>
                  <p>No data available. See documentation for enabling the metric names tracker.</p>
                </>
              )}
            </div>}
        >
          <div className="vm-cardinality-totals-card__info-icon"><InfoOutlinedIcon/></div>
        </Tooltip>
        Total metric names
      </h4>
      <span className="vm-cardinality-totals-card__value">{total}</span>
      <span className="vm-cardinality-totals-card__link">
        <Hyperlink href="https://docs.victoriametrics.com/victoriametrics/single-node-version/#track-ingested-metrics-usage">Docs ↗</Hyperlink>
      </span>
    </div>
  );
};

export default CardinalityMetricNameStats;
