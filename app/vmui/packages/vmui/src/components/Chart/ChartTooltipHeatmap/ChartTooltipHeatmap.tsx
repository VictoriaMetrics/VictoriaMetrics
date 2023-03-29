import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot from "uplot";
import ReactDOM from "react-dom";
import Button from "../../Main/Button/Button";
import { CloseIcon, DragIcon } from "../../Main/Icons";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent } from "react";
import "../ChartTooltip/style.scss";

export interface TooltipHeatmapProps  {
  cursor: {left: number, top: number}
  startDate: string,
  endDate: string,
  metricName: string,
  fields: string[],
  value: number,
  valueFormat: string
}

export interface ChartTooltipHeatmapProps extends TooltipHeatmapProps {
  id: string,
  u: uPlot,
  unit?: string,
  isSticky?: boolean,
  tooltipOffset: { left: number, top: number },
  onClose?: (id: string) => void
}

const ChartTooltipHeatmap: FC<ChartTooltipHeatmapProps> = ({
  u,
  id,
  unit = "",
  cursor,
  tooltipOffset,
  isSticky,
  onClose,
  startDate,
  endDate,
  metricName,
  fields,
  valueFormat,
  value
}) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const [position, setPosition] = useState({ top: -999, left: -999 });
  const [moving, setMoving] = useState(false);
  const [moved, setMoved] = useState(false);

  const targetPortal = useMemo(() => u.root.querySelector(".u-wrap"), [u]);

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

    const topOnChart = cursor.top;
    const leftOnChart = cursor.left;
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

  useEffect(calcPosition, [u, cursor, tooltipOffset, tooltipRef]);

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

  if (!targetPortal || !cursor.left || !cursor.top || !value) return null;

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
        <div className="vm-chart-tooltip-header__date vm-chart-tooltip-header__date_range">
          <span>{startDate}</span>
          <span>{endDate}</span>
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
        <p>
          {metricName}:
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

export default ChartTooltipHeatmap;
