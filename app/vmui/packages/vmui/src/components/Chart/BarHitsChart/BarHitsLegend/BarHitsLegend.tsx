import React, { FC, useEffect, useState } from "preact/compat";
import uPlot, { Series } from "uplot";
import "./style.scss";
import "../../Line/Legend/style.scss";
import BarHitsLegendItem from "./BarHitsLegendItem";
import { LegendLogHits } from "../../../../api/types";

interface Props {
  uPlotInst: uPlot;
  legendDetails: LegendLogHits[];
  onApplyFilter: (value: string) => void;
}

const BarHitsLegend: FC<Props> = ({ uPlotInst, legendDetails, onApplyFilter }) => {
  const [series, setSeries] = useState<Series[]>([]);
  const totalHits = legendDetails[0]?.totalHits || 0;

  const getSeries = () => {
    return uPlotInst.series.filter(s => s.scale !== "x");
  };

  const handleRedrawGraph = () => {
    uPlotInst.redraw();
    setSeries(getSeries());
  };

  useEffect(() => {
    setSeries(getSeries());
  }, [uPlotInst]);

  return (
    <div className="vm-bar-hits-legend">
      {legendDetails.map((legend) => (
        <BarHitsLegendItem
          key={legend.label}
          legend={legend}
          series={series}
          onRedrawGraph={handleRedrawGraph}
          onApplyFilter={onApplyFilter}
        />
      ))}
      <div className="vm-bar-hits-legend-info">
        <div>
          Total hits: <b>{totalHits.toLocaleString("en-US")}</b>
        </div>
        <div>
          <code>L-Click</code> toggles visibility.&nbsp;
          <code>R-Click</code> opens menu.
        </div>
      </div>
    </div>
  );
};

export default BarHitsLegend;
