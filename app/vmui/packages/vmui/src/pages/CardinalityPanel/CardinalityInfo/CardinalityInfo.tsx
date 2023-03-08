import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import { InfoIcon } from "../../../components/Main/Icons";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import {
  TipCardinalityOfLabel,
  TipCardinalityOfSingle, TipDocumentation,
  TipHighNumberOfSeries,
  TipHighNumberOfValues
} from "../CardinalityTips";
import { TopHeapEntry } from "../types";
import { useSearchParams } from "react-router-dom";
import classNames from "classnames";

interface CardinalityHeaderProps {
  totalSeries: number;
  totalSeriesAll: number;
  totalLabelValuePairs: number;
  seriesCountByMetricName: TopHeapEntry[];
}

const CardinalityInfo: FC<CardinalityHeaderProps> = ({
  totalSeries,
  totalSeriesAll,
  totalLabelValuePairs,
  seriesCountByMetricName
}) => {
  const [searchParams] = useSearchParams();
  const date = searchParams.get("date");
  const match = searchParams.get("match");
  const focusLabel = searchParams.get("focusLabel");
  const isMetric = /__name__/.test(match || "");

  const progress = seriesCountByMetricName[0]?.value / totalSeriesAll * 100;

  const matchValue = useMemo(() => {
    if (!match) return "-";
    if (isMetric) {
      return match.replace(/{.+="(.+)"}/, "$1");
    }
    return match.replace(/[{}]/g, "");
  }, [match, isMetric]);

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
      title: isMetric ? "Metric" : "label=value",
      value: matchValue,
      display: !!match && !focusLabel,
      classModifier: "metric",
    },
    {
      title: "Percentage of total series",
      value: isNaN(progress) ? "-" : `${progress.toFixed(2)}%`,
      display: isMetric,
      info: `Count all time series with metric name testrr and express that as a percentage 
             of the total number of time series in this data source.`
    },
    {
      title: "Date",
      value: date,
      display: true,
    },
    {
      title: "Number of entries",
      value: seriesCountByMetricName.length,
      display: true,
    }
  ].filter(t => t.display);

  return (
    <div className="vm-cardinality-info">
      {!!totals.length && (
        <div className="vm-cardinality-info-totals">
          {totals.map(({ title, value, info, classModifier }) => (
            <div
              className={classNames({
                "vm-cardinality-info-card": true,
                [`vm-cardinality-info-card_${classModifier}`]: classModifier,
              })}
              key={title}
            >
              <div className="vm-cardinality-info-card-header">
                {info && (
                  <Tooltip title={<p className="vm-cardinality-info-card-header__tooltip">{info}</p>}>
                    <div className="vm-cardinality-info-card-header__info-icon"><InfoIcon/></div>
                  </Tooltip>
                )}
                <h4 className="vm-cardinality-info-card-header__title">{title}</h4>
              </div>
              <span className="vm-cardinality-info-card__value">{value}</span>
            </div>
          ))}
        </div>
      )}

      <TipDocumentation/>
      {!match && !focusLabel && <TipHighNumberOfSeries/>}
      {match && !focusLabel && <TipCardinalityOfSingle/>}
      {!match && !focusLabel && <TipHighNumberOfValues/>}
      {focusLabel && <TipCardinalityOfLabel/>}
    </div>
  );
};

export default CardinalityInfo;
