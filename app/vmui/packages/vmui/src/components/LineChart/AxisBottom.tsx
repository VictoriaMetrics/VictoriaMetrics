import React, {useEffect, useRef} from "react";
import {axisBottom, ScaleTime, select as d3Select} from "d3";

interface AxisBottomI {
  xScale: ScaleTime<number, number>;
  height: number;
}

export const AxisBottom: React.FC<AxisBottomI> = ({xScale, height}) => {
  //eslint-disable-next-line @typescript-eslint/no-explicit-any
  const ref = useRef<SVGSVGElement | any>(null);

  useEffect(() => {
    d3Select(ref.current)
      .call(axisBottom<Date>(xScale));
  }, [xScale]);
  return <g ref={ref} className="x axis" transform={`translate(0, ${height})`} />;
};
