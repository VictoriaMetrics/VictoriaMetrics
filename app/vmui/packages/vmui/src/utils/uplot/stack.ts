// taken from https://github.com/leeoniya/uPlot/blob/master/demos/stack.js

import { AlignedData, Band } from "uplot";

function stack(data: AlignedData, omit: (i: number) => boolean) {
  const data2 = [];
  let bands = [];
  const d0Len = data[0].length;
  const accum = Array(d0Len);

  for (let i = 0; i < d0Len; i++)
    accum[i] = 0;

  for (let i = 1; i < data.length; i++)
    data2.push(omit(i) ? data[i] : data[i].map((v, i) => (accum[i] += +(v ?? 0))));

  for (let i = 1; i < data.length; i++)
    !omit(i) && bands.push({
      series: [
        data.findIndex((_s, j) => j > i && !omit(j)),
        i,
      ],
    });

  bands = bands.filter(b => b.series[1] > -1);

  return {
    data: [data[0]].concat(data2) as AlignedData,
    bands: bands as Band[],
  };
}

export default stack;
