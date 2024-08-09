import React, { FC, useMemo, useRef, useState } from "preact/compat";
import "./style.scss";
import "uplot/dist/uPlot.min.css";
import useElementSize from "../../../hooks/useElementSize";
import uPlot, { AlignedData } from "uplot";
import { useEffect } from "react";
import useBarHitsOptions from "./hooks/useBarHitsOptions";
import BarHitsTooltip from "./BarHitsTooltip/BarHitsTooltip";
import { TimeParams } from "../../../types";
import usePlotScale from "../../../hooks/uplot/usePlotScale";
import useReadyChart from "../../../hooks/uplot/useReadyChart";
import useZoomChart from "../../../hooks/uplot/useZoomChart";
import classNames from "classnames";
import { LogHits } from "../../../api/types";
import { addSeries, delSeries, setBand } from "../../../utils/uplot";
import { GraphOptions, GRAPH_STYLES } from "./types";
import BarHitsOptions from "./BarHitsOptions/BarHitsOptions";
import stack from "../../../utils/uplot/stack";
import BarHitsLegend from "./BarHitsLegend/BarHitsLegend";

interface Props {
  logHits: LogHits[];
  data: AlignedData;
  period: TimeParams;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  onApplyFilter: (value: string) => void;
}
const BarHitsChart: FC<Props> = ({ logHits, data: _data, period, setPeriod, onApplyFilter }) => {
  const [containerRef, containerSize] = useElementSize();
  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const [graphOptions, setGraphOptions] = useState<GraphOptions>({
    graphStyle: GRAPH_STYLES.LINE_STEPPED,
    stacked: false,
    fill: false,
  });

  const { xRange, setPlotScale } = usePlotScale({ period, setPeriod });
  const { onReadyChart, isPanning } = useReadyChart(setPlotScale);
  useZoomChart({ uPlotInst, xRange, setPlotScale });

  const { data, bands } = useMemo(() => {
    return graphOptions.stacked ? stack(_data, () => false) : { data: _data, bands: [] };
  }, [graphOptions, _data]);

  const { options, series, focusDataIdx } = useBarHitsOptions({
    data,
    logHits,
    bands,
    xRange,
    containerSize,
    onReadyChart,
    setPlotScale,
    graphOptions
  });

  useEffect(() => {
    if (!uPlotInst) return;
    delSeries(uPlotInst);
    addSeries(uPlotInst, series, true);
    setBand(uPlotInst, series);
    uPlotInst.redraw();
  }, [series]);

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
    <div className="vm-bar-hits-chart__wrapper">
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
        <BarHitsTooltip
          uPlotInst={uPlotInst}
          data={_data}
          focusDataIdx={focusDataIdx}
        />
      </div>
      <BarHitsOptions onChange={setGraphOptions}/>
      {uPlotInst && (
        <BarHitsLegend
          uPlotInst={uPlotInst}
          onApplyFilter={onApplyFilter}
        />
      )}
    </div>
  );
};

export default BarHitsChart;
