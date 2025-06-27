import React, { FC, useCallback } from "preact/compat";
import useElementSize from "../../../../hooks/useElementSize";
import { useEffect, useMemo, useRef, useState } from "react";
import uPlot, { AlignedData } from "uplot";
import { GraphOptions } from "../types";
import usePlotScale from "../../../../hooks/uplot/usePlotScale";
import useReadyChart from "../../../../hooks/uplot/useReadyChart";
import useZoomChart from "../../../../hooks/uplot/useZoomChart";
import stack from "../../../../utils/uplot/stack";
import useBarHitsOptions, { getLabelFromLogHit } from "../hooks/useBarHitsOptions";
import { LegendLogHits, LogHits } from "../../../../api/types";
import { addSeries, delSeries, setBand } from "../../../../utils/uplot";
import classNames from "classnames";
import BarHitsTooltip from "../BarHitsTooltip/BarHitsTooltip";
import { TimeParams } from "../../../../types";
import BarHitsLegend from "../BarHitsLegend/BarHitsLegend";
import { calculateTotalHits, sortLogHits } from "../../../../utils/logs";

interface Props {
  logHits: LogHits[];
  data: AlignedData;
  period: TimeParams;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  onApplyFilter: (value: string) => void;
  graphOptions: GraphOptions;
}

const BarHitsPlot: FC<Props> = ({ graphOptions, logHits, data: _data, period, setPeriod, onApplyFilter }: Props) => {
  const [containerRef, containerSize] = useElementSize();
  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();

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

  const prepareLegend = useCallback((hits: LogHits[], totalHits: number): LegendLogHits[] => {
    return hits.map((hit) => {
      const label = getLabelFromLogHit(hit);

      const legendItem: LegendLogHits = {
        label,
        isOther: hit._isOther,
        fields: hit.fields,
        total: hit.total || 0,
        totalHits,
        stroke: series.find((s) => s.label === label)?.stroke,
      };

      return legendItem;
    }).sort(sortLogHits("total"));
  }, [series]);


  const legendDetails: LegendLogHits[] = useMemo(() => {
    const totalHits = calculateTotalHits(logHits);
    return prepareLegend(logHits, totalHits);
  }, [logHits, prepareLegend]);

  useEffect(() => {
    if (!uPlotInst) return;

    const oldSeriesMap = new Map(uPlotInst.series.map(s => [s.label, s]));

    const syncedSeries = series.map(s => {
      const old = oldSeriesMap.get(s.label);
      return old ? { ...s, show: old.show } : s;
    });

    delSeries(uPlotInst);
    addSeries(uPlotInst, syncedSeries, true);
    setBand(uPlotInst, syncedSeries);
    uPlotInst.redraw();
  }, [series, uPlotInst]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.delBand();
    bands.forEach(band => {
      uPlotInst.addBand(band);
    });
    uPlotInst.redraw();
  }, [bands]);

  useEffect(() => {
    if (!uPlotRef.current) return;
    const uplot = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(uplot);
    return () => uplot.destroy();
  }, [uPlotRef.current]);

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
    <>
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
      {uPlotInst && <BarHitsLegend
        uPlotInst={uPlotInst}
        onApplyFilter={onApplyFilter}
        legendDetails={legendDetails}
      />}
    </>
  );
};

export default BarHitsPlot;
