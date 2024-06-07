import React, { FC, useMemo } from "preact/compat";
import { ForecastType, SeriesItem } from "../../../../types";
import { anomalyColors } from "../../../../utils/color";
import "./style.scss";

type Props = {
  series: SeriesItem[];
};

const titles: Partial<Record<ForecastType, string>> = {
  [ForecastType.yhat]: "yhat",
  [ForecastType.yhatLower]: "yhat_upper - yhat_lower",
  [ForecastType.yhatUpper]: "yhat_upper - yhat_lower",
  [ForecastType.anomaly]: "anomalies",
  [ForecastType.training]: "training data",
  [ForecastType.actual]: "y"
};

const LegendAnomaly: FC<Props> = ({ series }) => {

  const uniqSeriesStyles = useMemo(() => {
    const uniqSeries = series.reduce((accumulator, currentSeries) => {
      const hasForecast = Object.prototype.hasOwnProperty.call(currentSeries, "forecast");
      const isNotUpper = currentSeries.forecast !== ForecastType.yhatUpper;
      const isUniqForecast = !accumulator.find(s => s.forecast === currentSeries.forecast);
      if (hasForecast && isUniqForecast && isNotUpper) {
        accumulator.push(currentSeries);
      }
      return accumulator;
    }, [] as SeriesItem[]);

    const trainingSeries = {
      ...uniqSeries[0],
      forecast: ForecastType.training,
      color: anomalyColors[ForecastType.training],
    };
    uniqSeries.splice(1, 0, trainingSeries);

    return uniqSeries.map(s => ({
      ...s,
      color: typeof s.stroke === "string" ? s.stroke : anomalyColors[s.forecast || ForecastType.actual],
    }));
  }, [series]);

  return <>
    <div className="vm-legend-anomaly">
      {/* TODO: remove .filter() after the correct training data has been added */}
      {uniqSeriesStyles.filter(f => f.forecast !== ForecastType.training).map((s, i) => (
        <div
          key={`${i}_${s.forecast}`}
          className="vm-legend-anomaly-item"
        >
          <svg>
            {s.forecast === ForecastType.anomaly ? (
              <circle
                cx="15"
                cy="7"
                r="4"
                fill={s.color}
                stroke={s.color}
                strokeWidth="1.4"
              />
            ) : (
              <line
                x1="0"
                y1="7"
                x2="30"
                y2="7"
                stroke={s.color}
                strokeWidth={s.width || 1}
                strokeDasharray={s.dash?.join(",")}
              />
            )}
          </svg>
          <div className="vm-legend-anomaly-item__title">{titles[s.forecast || ForecastType.actual]}</div>
        </div>
      ))}
    </div>
  </>;
};

export default LegendAnomaly;
