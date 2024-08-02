import React, { FC, useRef, useState } from "preact/compat";
import "./style.scss";
import "uplot/dist/uPlot.min.css";
import useElementSize from "../../../hooks/useElementSize";
import uPlot, { AlignedData } from "uplot";
import { useEffect } from "react";
import useBarHitsOptions from "./hooks/useBarHitsOptions";
import TooltipBarHitsChart from "./TooltipBarHitsChart";
import { TimeParams } from "../../../types";
import usePlotScale from "../../../hooks/uplot/usePlotScale";
import useReadyChart from "../../../hooks/uplot/useReadyChart";
import useZoomChart from "../../../hooks/uplot/useZoomChart";
import classNames from "classnames";

interface Props {
  data: AlignedData;
  period: TimeParams;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
}

const BarHitsChart: FC<Props> = ({ data, period, setPeriod }) => {
  const [containerRef, containerSize] = useElementSize();
  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();

  const { xRange, setPlotScale } = usePlotScale({ period, setPeriod });
  const { onReadyChart, isPanning } = useReadyChart(setPlotScale);
  useZoomChart({ uPlotInst, xRange, setPlotScale });
  const { options, focusDataIdx } = useBarHitsOptions({ xRange, containerSize, onReadyChart, setPlotScale });

  useEffect(() => {
    if (!uPlotRef.current) return;
    const uplot = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(uplot);
    return () => uplot.destroy();
  }, [uPlotRef.current, options]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.scales.x.range = () => [xRange.min, xRange.max];
    uPlotInst.redraw();
  }, [xRange]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.setSize(containerSize);
    uPlotInst.redraw();
  }, [containerSize]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.setData(data);
    uPlotInst.redraw();
  }, [data]);

  return (
    <div
      className={classNames({
        "vm-bar-hits-chart": true,
        "vm-bar-hits-chart_panning": isPanning
      })}
      ref={containerRef}
    >
      <div
        className="vm-line-chart__u-plot"
        ref={uPlotRef}
      />
      <TooltipBarHitsChart
        uPlotInst={uPlotInst}
        focusDataIdx={focusDataIdx}
      />
    </div>
  );
};

export default BarHitsChart;
