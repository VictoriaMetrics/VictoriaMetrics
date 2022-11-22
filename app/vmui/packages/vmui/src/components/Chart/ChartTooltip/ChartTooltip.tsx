import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot, { Series } from "uplot";
import { MetricResult } from "../../../api/types";
import { formatPrettyNumber, getColorLine, getLegendLabel } from "../../../utils/uplot/helpers";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../../constants/date";
import ReactDOM from "react-dom";
import get from "lodash.get";
import Button from "../../Main/Button/Button";
import { CloseIcon, DragIcon } from "../../Main/Icons";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent } from "react";
import "./style.scss";

export interface ChartTooltipProps {
  id: string,
  u: uPlot,
  metrics: MetricResult[],
  series: Series[],
  unit?: string,
  isSticky?: boolean,
  tooltipOffset: { left: number, top: number },
  tooltipIdx: { seriesIdx: number, dataIdx: number },
  onClose?: (id: string) => void
}

const ChartTooltip: FC<ChartTooltipProps> = ({
  u,
  id,
  unit = "",
  metrics,
  series,
  tooltipIdx,
  tooltipOffset,
  isSticky,
  onClose
}) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const [position, setPosition] = useState({ top: -999, left: -999 });
  const [moving, setMoving] = useState(false);
  const [moved, setMoved] = useState(false);

  const [seriesIdx, setSeriesIdx] = useState(tooltipIdx.seriesIdx);
  const [dataIdx, setDataIdx] = useState(tooltipIdx.dataIdx);

  const targetPortal = useMemo(() => u.root.querySelector(".u-wrap"), [u]);

  const value = useMemo(() => get(u, ["data", seriesIdx, dataIdx], 0), [u, seriesIdx, dataIdx]);
  const valueFormat = useMemo(() => formatPrettyNumber(value), [value]);
  const dataTime = useMemo(() => u.data[0][dataIdx], [u, dataIdx]);
  const date = useMemo(() => dayjs(new Date(dataTime * 1000)).format(DATE_FULL_TIMEZONE_FORMAT), [dataTime]);

  const color = useMemo(() => getColorLine(series[seriesIdx]?.label || ""), [series, seriesIdx]);

  const name = useMemo(() => {
    const metricName = (series[seriesIdx]?.label || "").replace(/{.+}/gmi, "").trim();
    return getLegendLabel(metricName);
  }, []);

  const fields = useMemo(() => {
    const metric = metrics[seriesIdx - 1]?.metric || {};
    const fields = Object.keys(metric).filter(k => k !== "__name__");
    return fields.map(key => `${key}="${metric[key]}"`);
  }, [metrics, seriesIdx]);

  const handleClose = () => {
    onClose && onClose(id);
  };

  const handleMouseDown = (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => {
    setMoved(true);
    setMoving(true);
    const { clientX, clientY } = e;
    setPosition({ top: clientY, left: clientX });
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!moving) return;
    const { clientX, clientY } = e;
    setPosition({ top: clientY, left: clientX });
  };

  const handleMouseUp = () => {
    setMoving(false);
  };

  const calcPosition = () => {
    if (!tooltipRef.current) return;

    const topOnChart = u.valToPos((value || 0), series[seriesIdx]?.scale || "1");
    const leftOnChart = u.valToPos(dataTime, "x");
    const { width: tooltipWidth, height: tooltipHeight } = tooltipRef.current.getBoundingClientRect();
    const { width, height } = u.over.getBoundingClientRect();

    const margin = 10;
    const overflowX = leftOnChart + tooltipWidth >= width ? tooltipWidth + (2 * margin) : 0;
    const overflowY = topOnChart + tooltipHeight >= height ? tooltipHeight + (2 * margin) : 0;

    setPosition({
      top: topOnChart + tooltipOffset.top + margin - overflowY,
      left: leftOnChart + tooltipOffset.left + margin - overflowX
    });
  };

  useEffect(calcPosition, [u, value, dataTime, seriesIdx, tooltipOffset, tooltipRef]);

  useEffect(() => {
    setSeriesIdx(tooltipIdx.seriesIdx);
    setDataIdx(tooltipIdx.dataIdx);
  }, [tooltipIdx]);

  useEffect(() => {
    if (moving) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
    }

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [moving]);

  if (!targetPortal || tooltipIdx.seriesIdx < 0 || tooltipIdx.dataIdx < 0) return null;

  return ReactDOM.createPortal((
    <div
      className={classNames({
        "vm-chart-tooltip": true,
        "vm-chart-tooltip_sticky": isSticky,
        "vm-chart-tooltip_moved": moved

      })}
      ref={tooltipRef}
      style={position}
    >
      <div className="vm-chart-tooltip-header">
        <div className="vm-chart-tooltip-header__date">{date}</div>
        {isSticky && (
          <>
            <Button
              className="vm-chart-tooltip-header__drag"
              variant="text"
              size="small"
              startIcon={<DragIcon/>}
              onMouseDown={handleMouseDown}
            />
            <Button
              className="vm-chart-tooltip-header__close"
              variant="text"
              size="small"
              startIcon={<CloseIcon/>}
              onClick={handleClose}
            />
          </>
        )}
      </div>
      <div className="vm-chart-tooltip-data">
        <div
          className="vm-chart-tooltip-data__marker"
          style={{ background: color }}
        />
        <p>
          {name}:
          <b className="vm-chart-tooltip-data__value">{valueFormat}</b>
          {unit}
        </p>
      </div>
      {!!fields.length && (
        <div className="vm-chart-tooltip-info">
          {fields.map((f, i) => (
            <div key={`${f}_${i}`}>{f}</div>
          ))}
        </div>
      )}
    </div>
  ), targetPortal);
};

export default ChartTooltip;
