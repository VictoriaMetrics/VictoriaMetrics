import React, {FC, useMemo} from "react";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";
import "../../../utils/chartjs-register-plugins";

export interface GraphViewProps {
  data?: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data = []}) => {

  const amountOfSeries = useMemo(() => data.length, [data]);

  return <>
    {(amountOfSeries > 0)
      ? <LineChart data={data} />
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;