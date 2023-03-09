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
  totalLabelValuePairs,
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
      info: `The total number of active time series in the selected data source. 
             A time series is a unique combination of a metric name and key-value label pairs. 
             For example, "events_totalenv=dev!" and "events_total{env=prod}" are two distinct time series, 
             both of which belong to the same parent metric, "events_total."`
    },
    {
      title: "Total label value pairs",
      value: totalLabelValuePairs.toLocaleString("en-US"),
      display: !match && !focusLabel,
      info: `Labels are key<>value pairs. 
             "Total unique label value pairs" is the count of unique labels in the selected data source. 
             The word "unique" If y emphasizes that if the same label (e.g., "env=dev") 
             is applied to every uni time series in your system, 
             it would still only increase your count of "total unique label values pairs" by one.`
    },
    {
      title: "Percentage of total series",
      value: isNaN(progress) ? "-" : `${progress.toFixed(2)}%`,
      display: isMetric,
      info: `Count all time series with metric name testrr and express that as a percentage 
             of the total number of time series in this data source.`
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
