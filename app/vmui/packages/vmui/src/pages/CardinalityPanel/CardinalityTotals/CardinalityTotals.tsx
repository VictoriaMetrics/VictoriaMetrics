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
  totalLabelValuePairs: number;
  seriesCountByMetricName: TopHeapEntry[];
}

const CardinalityTotals: FC<CardinalityTotalsProps> = ({
  totalSeries,
  totalSeriesAll,
  seriesCountByMetricName
}) => {
  const { isMobile } = useDeviceDetect();

  const [searchParams] = useSearchParams();
  const match = searchParams.get("match");
  const focusLabel = searchParams.get("focusLabel");
  const isMetric = /__name__/.test(match || "");

  const progress = seriesCountByMetricName[0]?.value / totalSeriesAll * 100;

  const totals = [
    {
      title: "Total series",
      value: totalSeries.toLocaleString("en-US"),
      display: !focusLabel,
      info: `The total number of active time series. 
             A time series is uniquely identified by its name plus a set of its labels. 
             For example, temperature{city="NY",country="US"} and temperature{city="SF",country="US"} 
             are two distinct series, since they differ by the city label.`
    },
    {
      title: "Percentage of total series",
      value: isNaN(progress) ? "-" : `${progress.toFixed(2)}%`,
      display: isMetric,
      info: `Count all time series with metric name and express that as a percentage 
             of the total number of time series in data source.`
    }
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
      {totals.map(({ title, value, info }) => (
        <div
          className="vm-cardinality-totals-card"
          key={title}
        >
          <div className="vm-cardinality-totals-card-header">
            {info && (
              <Tooltip title={<p className="vm-cardinality-totals-card-header__tooltip">{info}</p>}>
                <div className="vm-cardinality-totals-card-header__info-icon"><InfoIcon/></div>
              </Tooltip>
            )}
            <h4 className="vm-cardinality-totals-card-header__title">{title}</h4>
          </div>
          <span className="vm-cardinality-totals-card__value">{value}</span>
        </div>
      ))}
    </div>
  );
};

export default CardinalityTotals;
