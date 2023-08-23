import React, { FC, useCallback, useEffect, useRef, useState } from "preact/compat";
import { MouseEvent as ReactMouseEvent } from "react";
import useEventListener from "../../../hooks/useEventListener";
import ReactDOM from "react-dom";
import classNames from "classnames";
import uPlot from "uplot";
import Button from "../../Main/Button/Button";
import { CloseIcon, DragIcon } from "../../Main/Icons";
import { SeriesItemStats } from "../../../types";

export interface ChartTooltipProps {
  u?: uPlot;
  id: string;
  title?: string;
  dates: string[];
  value: string | number | null;
  point: { top: number, left: number };
  unit?: string;
  stats?: SeriesItemStats;
  isSticky?: boolean;
  info?: string;
  marker?: string;
  show?: boolean;
  onClose?: (id: string) => void;
}

const ChartTooltip: FC<ChartTooltipProps> = ({
  u,
  id,
  title,
  dates,
  value,
  point,
  unit = "",
  info,
  stats,
  isSticky,
  marker,
  onClose
}) => {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const [position, setPosition] = useState({ top: -999, left: -999 });
  const [moving, setMoving] = useState(false);
  const [moved, setMoved] = useState(false);

  const handleClose = () => {
    onClose && onClose(id);
  };

  const handleMouseDown = (e: ReactMouseEvent) => {
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
    if (!tooltipRef.current || !u) return;

    const { top, left } = point;
    const uPlotPosition = {
      left: parseFloat(u.over.style.left),
      top: parseFloat(u.over.style.top)
    };

    const {
      width: uPlotWidth,
      height: uPlotHeight
    } = u.over.getBoundingClientRect();

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

    setPosition(position);
  };

  useEffect(calcPosition, [u, value, point, tooltipRef]);

  useEventListener("mousemove", handleMouseMove);
  useEventListener("mouseup", handleMouseUp);

  if (!u) return null;

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
        {title && (
          <div className="vm-chart-tooltip-header__title">
            {title}
          </div>
        )}
        <div className="vm-chart-tooltip-header__date">
          {dates.map((date, i) => <span key={i}>{date}</span>)}
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
        {marker && (
          <span
            className="vm-chart-tooltip-data__marker"
            style={{ background: marker }}
          />
        )}
        <div>
          <p className="vm-chart-tooltip-data__value">
            <b>{value}</b>{unit}
          </p>
          {stats && (
            <p className="vm-chart-tooltip-data__stats">
              {Object.keys(stats).filter(key => key !== "last").map((key, i) => (
                <span key={i}>
                  {key}:<b>{stats[key as keyof SeriesItemStats]}</b>
                </span>
              )
              )}
            </p>
          )}
        </div>
      </div>
      <div className="vm-chart-tooltip-info">
        {info}
      </div>
    </div>
  ), u.root);
};

export default ChartTooltip;
