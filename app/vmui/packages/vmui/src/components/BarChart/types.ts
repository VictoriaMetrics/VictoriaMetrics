import {AlignedData as uPlotData, Options as uPlotOptions} from "uplot";

export interface BarChartProps {
  data: uPlotData;
  container: HTMLDivElement | null,
  configs: uPlotOptions,
}
