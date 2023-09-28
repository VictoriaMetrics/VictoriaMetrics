import React, { FC } from "preact/compat";
import { InfoIcon } from "../../../components/Main/Icons";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { TopHeapEntry } from "../types";
import { useSearchParams } from "react-router-dom";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import "./style.scss";

export interface CardinalityTotalsProps {
  totalSeries: number;
  totalSeriesAll: number;
  totalSeriesPrev: number;
  totalLabelValuePairs: number;
  seriesCountByMetricName: TopHeapEntry[];
  isPrometheus?: boolean;
  isCluster: boolean;
}

const CardinalityTotals: FC<CardinalityTotalsProps> = ({
  totalSeries = 0,
  totalSeriesPrev = 0,
  totalSeriesAll = 0,
  seriesCountByMetricName = [],
  isPrometheus,
}) => {
  const { isMobile } = useDeviceDetect();

  const [searchParams] = useSearchParams();
  const match = searchParams.get("match");
  const focusLabel = searchParams.get("focusLabel");
  const isMetric = /__name__/.test(match || "");

  const progress = seriesCountByMetricName[0]?.value / totalSeriesAll * 100;
  const diff = totalSeries - totalSeriesPrev;
  const dynamic = Math.abs(diff) / totalSeriesPrev * 100;

  const totals = [
    {
      title: "Total series",
      value: totalSeries.toLocaleString("en-US"),
      dynamic: (!totalSeries || !totalSeriesPrev || isPrometheus) ? "" : `${dynamic.toFixed(2)}%`,
      display: !focusLabel,
      info: `The total number of active time series. 
             A time series is uniquely identified by its name plus a set of its labels. 
             For example, temperature{city="NY",country="US"} and temperature{city="SF",country="US"} 
             are two distinct series, since they differ by the city label.`
    },
    {
      title: "Percentage from total",
      value: isNaN(progress) ? "-" : `${progress.toFixed(2)}%`,
      display: isMetric,
      info: "The share of these series in the total number of time series."
    },
  ].filter(t => t.display);

  if (!totals.length) {
    return null;
  }

  return (
    <div
      className={classNames({
        "vm-cardinality-totals": true,
        "vm-cardinality-totals_mobile": isMobile
      })}
    >
      {totals.map(({ title, value, info, dynamic }) => (
        <div
          className="vm-cardinality-totals-card"
          key={title}
        >
          <h4 className="vm-cardinality-totals-card__title">
            {title}
            {info && (
              <Tooltip title={<p className="vm-cardinality-totals-card__tooltip">{info}</p>}>
                <div className="vm-cardinality-totals-card__info-icon"><InfoIcon/></div>
              </Tooltip>
            )}
          </h4>
          <span className="vm-cardinality-totals-card__value">{value}</span>
          {!!dynamic && (
            <Tooltip title={`in relation to the previous day: ${totalSeriesPrev.toLocaleString("en-US")}`}>
              <span
                className={classNames({
                  "vm-dynamic-number": true,
                  "vm-dynamic-number_positive vm-dynamic-number_down": diff < 0,
                  "vm-dynamic-number_negative vm-dynamic-number_up": diff > 0,
                })}
              >
                {dynamic}
              </span>
            </Tooltip>
          )}
        </div>
      ))}
    </div>
  );
};

export default CardinalityTotals;
