import React, {FC} from "react";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";

export interface GraphViewProps {
  data?: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data = []}) => {
  return <>
    {(data.length > 0)
      ? <LineChart data={data} />
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;