import React, { FC, useMemo, useRef } from "preact/compat";
import uPlot, { AlignedData } from "uplot";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import classNames from "classnames";
import "./style.scss";
import "../../ChartTooltip/style.scss";
import { sortLogHits } from "../../../../utils/logs";

interface Props {
  data: AlignedData;
  uPlotInst?: uPlot;
  focusDataIdx: number;
}

const timeFormat = (ts: number) => dayjs(ts * 1000).tz().format(DATE_TIME_FORMAT);

const BarHitsTooltip: FC<Props> = ({ data, focusDataIdx, uPlotInst }) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const tooltipData = useMemo(() => {
    const series = uPlotInst?.series || [];
    const [time, ...values] = data.map((d) => d[focusDataIdx] || 0);
    const step = (data[0][1] - data[0][0]);
    const timeNext = time + step;

    const tooltipItems = values.map((value, i) => {
      const targetSeries = series[i + 1];
      const stroke = (targetSeries?.stroke as () => string)?.();
      const label = targetSeries?.label;
      const show = targetSeries?.show;
      return {
        label,
        stroke,
        value,
        show
      };
    }).filter(item => item.value > 0 && item.show).sort(sortLogHits("value"));

    const point = {
      top: tooltipItems[0] ? uPlotInst?.valToPos?.(tooltipItems[0].value, "y") || 0 : 0,
      left: uPlotInst?.valToPos?.(time, "x") || 0,
    };

    return {
      point,
      values: tooltipItems,
      total: tooltipItems.reduce((acc, item) => acc + item.value, 0),
      timestamp: `${timeFormat(time)} - ${timeFormat(timeNext)}`,
    };
  }, [focusDataIdx, uPlotInst, data]);

  const tooltipPosition = useMemo(() => {
    if (!uPlotInst || !tooltipData.total || !tooltipRef.current) return;

    const { top, left } = tooltipData.point;
    const uPlotPosition = {
      left: parseFloat(uPlotInst.over.style.left),
      top: parseFloat(uPlotInst.over.style.top)
    };

    const {
      width: uPlotWidth,
      height: uPlotHeight
    } = uPlotInst.over.getBoundingClientRect();

    const {
      width: tooltipWidth,
      height: tooltipHeight
    } = tooltipRef.current.getBoundingClientRect();

    const margin = 50;
    const overflowX = left + tooltipWidth >= uPlotWidth ? tooltipWidth + (2 * margin) : 0;
    const overflowY = top + tooltipHeight >= uPlotHeight ? tooltipHeight + (2 * margin) : 0;

    const position = {
      top: top + uPlotPosition.top + margin - overflowY,
      left: left + uPlotPosition.left + margin - overflowX
    };

    if (position.left < 0) position.left = 20;
    if (position.top < 0) position.top = 20;

    return position;
  }, [tooltipData, uPlotInst, tooltipRef.current]);

  return (
    <div
      className={classNames({
        "vm-chart-tooltip": true,
        "vm-chart-tooltip_hits": true,
        "vm-bar-hits-tooltip": true,
        "vm-bar-hits-tooltip_visible": focusDataIdx !== -1 && tooltipData.values.length
      })}
      ref={tooltipRef}
      style={tooltipPosition}
    >
      <div>
        {tooltipData.values.map((item, i) => (
          <div
            className="vm-chart-tooltip-data"
            key={i}
          >
            <span
              className="vm-chart-tooltip-data__marker"
              style={{ background: item.stroke }}
            />
            <p className="vm-bar-hits-tooltip-item">
              <span className="vm-bar-hits-tooltip-item__label">{item.label}</span>
              <span>{item.value.toLocaleString("en-US")}</span>
            </p>
          </div>
        ))}
      </div>
      {tooltipData.values.length > 1 && (
        <div className="vm-chart-tooltip-data">
          <span/>
          <p className="vm-bar-hits-tooltip-item">
            <span className="vm-bar-hits-tooltip-item__label">Total</span>
            <span>{tooltipData.total.toLocaleString("en-US")}</span>
          </p>
        </div>
      )}
      <div className="vm-chart-tooltip-header">
        <div className="vm-chart-tooltip-header__title vm-bar-hits-tooltip__date">
          {tooltipData.timestamp}
        </div>
      </div>
    </div>
  );
};

export default BarHitsTooltip;
