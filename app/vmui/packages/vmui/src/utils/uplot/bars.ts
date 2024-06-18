import uPlot from "uplot";
import { LOGS_BARS_VIEW } from "../../constants/logs";

export const barPaths = (
  u: uPlot,
  seriesIdx: number,
  idx0: number,
  idx1: number,
): uPlot.Series.Paths | null => {
  const barSize = (u.under.clientWidth/LOGS_BARS_VIEW ) - 1;
  const barsPathBuilderFactory = uPlot?.paths?.bars?.({ size: [0.96, barSize] });
  return barsPathBuilderFactory ? barsPathBuilderFactory(u, seriesIdx, idx0, idx1) : null;
};

