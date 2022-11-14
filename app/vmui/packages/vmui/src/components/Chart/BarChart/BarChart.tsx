import React, { FC, useEffect, useRef, useState } from "preact/compat";
import uPlot, { Options as uPlotOptions } from "uplot";
import useResize from "../../../hooks/useResize";
import { BarChartProps } from "./types";
import "./style.scss";

const BarChart: FC<BarChartProps> = ({
  data,
  container,
  configs }) => {

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const layoutSize = useResize(container);

  const options: uPlotOptions ={
    ...configs,
    width: layoutSize.width || 400,
  };

  const updateChart = (): void => {
    if (!uPlotInst) return;
    uPlotInst.setData(data);
  };

  useEffect(() => {
    if (!uPlotRef.current) return;
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    return u.destroy;
  }, [uPlotRef.current, layoutSize]);

  useEffect(() => updateChart(), [data]);

  return <div style={{ height: "100%" }}>
    <div ref={uPlotRef}/>
  </div>;
};

export default BarChart;
