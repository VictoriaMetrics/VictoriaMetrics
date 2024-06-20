import React, { FC, useMemo, useRef } from "preact/compat";
import uPlot from "uplot";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import classNames from "classnames";
import "./style.scss";
import "../../../components/Chart/ChartTooltip/style.scss";

interface Props {
  uPlotInst?: uPlot;
  focusDataIdx: number
}

const TooltipBarHitsChart: FC<Props> = ({ focusDataIdx, uPlotInst }) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const tooltipData = useMemo(() => {
    const value = uPlotInst?.data?.[1]?.[focusDataIdx];
    const timestamp = uPlotInst?.data?.[0]?.[focusDataIdx] || 0;
    const top = uPlotInst?.valToPos?.((value || 0), "y") || 0;
    const left = uPlotInst?.valToPos?.(timestamp, "x") || 0;

    return {
      point: { top, left },
      value,
      timestamp: dayjs(timestamp * 1000).tz().format(DATE_TIME_FORMAT),
    };
  }, [focusDataIdx, uPlotInst]);

  const tooltipPosition = useMemo(() => {
    if (!uPlotInst || !tooltipData.value || !tooltipRef.current) return;

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

    const margin = 10;
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
        "vm-bar-hits-chart-tooltip": true,
        "vm-bar-hits-chart-tooltip_visible": focusDataIdx !== -1
      })}
      ref={tooltipRef}
      style={tooltipPosition}
    >
      <div className="vm-chart-tooltip-data">
        Count of records:
        <p className="vm-chart-tooltip-data__value">
          <b>{tooltipData.value}</b>
        </p>
      </div>
      <div className="vm-chart-tooltip-header">
        <div className="vm-chart-tooltip-header__title">
          {tooltipData.timestamp}
        </div>
      </div>
    </div>
  );
};

export default TooltipBarHitsChart;
