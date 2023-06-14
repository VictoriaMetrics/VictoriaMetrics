import React, { FC, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot from "uplot";
import { MetricResult } from "../../../../api/types";
import { formatPrettyNumber } from "../../../../utils/uplot/helpers";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../../../constants/date";
import ReactDOM from "react-dom";
import get from "lodash.get";
import Button from "../../../Main/Button/Button";
import { CloseIcon, DragIcon } from "../../../Main/Icons";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent } from "react";
import "./style.scss";
import { SeriesItem } from "../../../../utils/uplot/series";
import useEventListener from "../../../../hooks/useEventListener";

export interface ChartTooltipProps {
  id: string,
  u: uPlot,
  metricItem: MetricResult,
  seriesItem: SeriesItem,
  unit?: string,
  isSticky?: boolean,
  showQueryNum?: boolean,
  tooltipOffset: { left: number, top: number },
  tooltipIdx: { seriesIdx: number, dataIdx: number },
  onClose?: (id: string) => void
}

const ChartTooltip: FC<ChartTooltipProps> = ({
  u,
  id,
  unit = "",
  metricItem,
  seriesItem,
  tooltipIdx,
  tooltipOffset,
  isSticky,
  showQueryNum,
  onClose
}) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const [position, setPosition] = useState({ top: -999, left: -999 });
  const [moving, setMoving] = useState(false);
  const [moved, setMoved] = useState(false);

  const [seriesIdx, setSeriesIdx] = useState(tooltipIdx.seriesIdx);
  const [dataIdx, setDataIdx] = useState(tooltipIdx.dataIdx);

  const value = get(u, ["data", seriesIdx, dataIdx], 0);
  const valueFormat = formatPrettyNumber(value, get(u, ["scales", "1", "min"], 0), get(u, ["scales", "1", "max"], 1));
  const dataTime = u.data[0][dataIdx];
  const date = dayjs(dataTime * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT);

  const color = `${seriesItem?.stroke}`;
  const calculations = seriesItem?.calculations || {};
  const group = metricItem?.group || 0;


  const fullMetricName = useMemo(() => {
    const metric = metricItem?.metric || {};
    const labelNames = Object.keys(metric).filter(x => x != "__name__");
    const labels = labelNames.map(key => `${key}=${JSON.stringify(metric[key])}`);
    let metricName = metric["__name__"] || "";
    if (labels.length > 0) {
      metricName += "{" + labels.join(",") + "}";
    }
    return metricName;
  }, [metricItem]);

  const handleClose = () => {
    onClose && onClose(id);
  };

  const handleMouseDown = (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => {
    setMoved(true);
    setMoving(true);
    const { clientX, clientY } = e;
    setPosition({ top: clientY, left: clientX });
  };

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!moving) return;
    const { clientX, clientY } = e;
    setPosition({ top: clientY, left: clientX });
  }, [moving]);

  const handleMouseUp = () => {
    setMoving(false);
  };

  const calcPosition = () => {
    if (!tooltipRef.current) return;

    const topOnChart = u.valToPos((value || 0), seriesItem?.scale || "1");
    const leftOnChart = u.valToPos(dataTime, "x");
    const { width: tooltipWidth, height: tooltipHeight } = tooltipRef.current.getBoundingClientRect();
    const { width, height } = u.over.getBoundingClientRect();

    const margin = 10;
    const overflowX = leftOnChart + tooltipWidth >= width ? tooltipWidth + (2 * margin) : 0;
    const overflowY = topOnChart + tooltipHeight >= height ? tooltipHeight + (2 * margin) : 0;

    const position = {
      top: topOnChart + tooltipOffset.top + margin - overflowY,
      left: leftOnChart + tooltipOffset.left + margin - overflowX
    };

    if (position.left < 0) position.left = 20;
    if (position.top < 0) position.top = 20;

    setPosition(position);
  };

  useEffect(calcPosition, [u, value, dataTime, seriesIdx, tooltipOffset, tooltipRef]);

  useEffect(() => {
    setSeriesIdx(tooltipIdx.seriesIdx);
    setDataIdx(tooltipIdx.dataIdx);
  }, [tooltipIdx]);

  useEventListener("mousemove", handleMouseMove);
  useEventListener("mouseup", handleMouseUp);

  if (tooltipIdx.seriesIdx < 0 || tooltipIdx.dataIdx < 0) return null;

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
        <div className="vm-chart-tooltip-header__date">
          {showQueryNum && (<div>Query {group}</div>)}
          {date}
        </div>
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
        <div>
          <b>{valueFormat}{unit}</b><br/>
          median:<b>{calculations.median}</b>, min:<b>{calculations.min}</b>, max:<b>{calculations.max}</b>
        </div>
      </div>
      <div className="vm-chart-tooltip-info">
        {fullMetricName}
      </div>
    </div>
  ), u.root);
};

export default ChartTooltip;
