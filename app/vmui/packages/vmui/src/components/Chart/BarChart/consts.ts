import { seriesBarsPlugin } from "../../../utils/uplot/plugin";
import { barDisp, getBarSeries } from "../../../utils/uplot";
import { Fill, Stroke } from "../../../types";
import { PaddingSide, Series } from "uplot";


const stroke: Stroke = {
  unit: 3,
  values: (u: { data: number[][]; }) => u.data[1].map((_: number, idx) =>
    idx !== 0 ? "#33BB55" : "#F79420"
  ),
};

const fill: Fill = {
  unit: 3,
  values: (u: { data: number[][]; }) => u.data[1].map((_: number, idx) =>
    idx !== 0 ? "#33BB55" : "#F79420"
  ),
};

export const barOptions = {
  height: 500,
  width: 500,
  padding: [null, 0, null, 0] as [top: PaddingSide, right: PaddingSide, bottom: PaddingSide, left: PaddingSide],
  axes: [{ show: false }],
  series: [
    {
      label: "",
      value: (u: uPlot, v: string) => v
    },
    {
      label: " ",
      width: 0,
      fill: "",
      values: (u: uPlot, seriesIdx: number) => {
        const idxs = u.legend.idxs || [];

        if (u.data === null || idxs.length === 0)
          return { "Name": null, "Value": null, };

        const dataIdx = idxs[seriesIdx] || 0;

        const build = u.data[0][dataIdx];
        const duration = u.data[seriesIdx][dataIdx];

        return { "Name": build, "Value": duration };
      }
    },
  ] as Series[],
  plugins: [seriesBarsPlugin(getBarSeries([1], 0, 1, 0, barDisp(stroke, fill)))],
};
