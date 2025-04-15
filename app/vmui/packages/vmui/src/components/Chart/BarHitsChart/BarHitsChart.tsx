import React, { FC, useState } from "preact/compat";
import "./style.scss";
import "uplot/dist/uPlot.min.css";
import { AlignedData } from "uplot";
import { TimeParams } from "../../../types";
import classNames from "classnames";
import { LogHits } from "../../../api/types";
import { GraphOptions, GRAPH_STYLES } from "./types";
import BarHitsOptions from "./BarHitsOptions/BarHitsOptions";
import BarHitsPlot from "./BarHitsPlot/BarHitsPlot";

interface Props {
  logHits: LogHits[];
  data: AlignedData;
  period: TimeParams;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  onApplyFilter: (value: string) => void;
}
const BarHitsChart: FC<Props> = ({ logHits, data: _data, period, setPeriod, onApplyFilter }) => {
  const [graphOptions, setGraphOptions] = useState<GraphOptions>({
    graphStyle: GRAPH_STYLES.LINE_STEPPED,
    stacked: false,
    fill: false,
    hideChart: false,
  });


  return (
    <div
      className={classNames({
        "vm-bar-hits-chart__wrapper": true,
        "vm-bar-hits-chart__wrapper_hidden": graphOptions.hideChart
      })}
    >
      {!graphOptions.hideChart && (
        <BarHitsPlot
          logHits={logHits}
          data={_data}
          period={period}
          setPeriod={setPeriod}
          onApplyFilter={onApplyFilter}
          graphOptions={graphOptions}
        />
      )}
      <BarHitsOptions onChange={setGraphOptions}/>
    </div>
  );
};

export default BarHitsChart;
