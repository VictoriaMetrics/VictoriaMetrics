import { AlignedData as uPlotData, Options as uPlotOptions } from "uplot";
import { ElementSize } from "../../../hooks/useElementSize";

export interface BarChartProps {
  data: uPlotData;
  layoutSize: ElementSize,
  configs: uPlotOptions,
}
