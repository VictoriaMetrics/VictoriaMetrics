import React, {FC, useMemo} from "react";
import {MetricResult} from "../../../api/types";
import {getNameForMetric} from "../../../utils/metric";
import Plot from "react-plotly.js";
import {Data as PlotData} from "plotly.js";
import {dateFromSeconds} from "../../../utils/time";

export interface GraphViewProps {
  data: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data}) => {
  const series: PlotData[] = useMemo(() => {
    return data?.map(d => ({
      name: getNameForMetric(d),
      x: d.values.map(v => dateFromSeconds(v[0])),
      y: d.values.map(v => +v[1]),
      type: "scatter",
      mode: "lines",
    }));
  }, [data]);

  const amountOfSeries = useMemo(() => series.length, [series]);

  const offsetLegend = useMemo(() => {
    // Plotly have problem with legend height
    if (amountOfSeries === 1) return -0.2;
    if (amountOfSeries >= 17) return -1.5;
    return (Math.round((amountOfSeries * 1.5)/2)) * -0.1;
  }, [amountOfSeries]);

  return <>
    {(amountOfSeries > 0)
      ? <>
        <Plot
          data={series}
          config={{scrollZoom: true}}
          layout={{
            height: 716,
            width: window.innerWidth - 80,
            dragmode: "pan",
            showlegend: true,
            margin: {l: 50, r: 4, b: 10, t: 40, pad: 10},
            legend: {
              x: 0,
              y: offsetLegend,
              xanchor: "left",
              font: {color: "#000"},
            }
          }}
        />
      </>
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;