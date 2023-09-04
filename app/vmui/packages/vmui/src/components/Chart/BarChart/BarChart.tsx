import React, { FC, useEffect, useRef, useState } from "preact/compat";
import uPlot, { Options as uPlotOptions } from "uplot";
import { BarChartProps } from "./types";
import "./style.scss";
import { useAppState } from "../../../state/common/StateContext";

const BarChart: FC<BarChartProps> = ({
  data,
  layoutSize,
  configs }) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();

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
  }, [uPlotRef.current, layoutSize, isDarkTheme]);

  useEffect(() => updateChart(), [data]);

  return <div style={{ height: "100%" }}>
    <div ref={uPlotRef}/>
  </div>;
};

export default BarChart;
