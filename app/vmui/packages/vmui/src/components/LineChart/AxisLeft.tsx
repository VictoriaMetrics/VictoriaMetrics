import React, {useEffect, useRef} from "react";
import {axisLeft, ScaleLinear, select as d3Select} from "d3";
import {format as d3Format} from "d3-format";

interface AxisLeftI {
  yScale: ScaleLinear<number, number>;
  label: string;
}

const yFormatter = (val: number): string => {
  const v = Math.abs(val); // helps to handle negatives the same way
  const DECIMAL_THRESHOLD = 0.001;
  let format = ".2~s"; // 21K tilde means that it won't be 2.0K but just 2K
  if (v > 0 && v < DECIMAL_THRESHOLD) {
    format = ".0e"; // 1E-3 for values below DECIMAL_THRESHOLD
  }
  if (v >= DECIMAL_THRESHOLD && v < 1) {
    format = ".3~f"; // just plain 0.932
  }
  return d3Format(format)(val);
};

export const AxisLeft: React.FC<AxisLeftI> = ({yScale, label}) => {
  //eslint-disable-next-line @typescript-eslint/no-explicit-any
  const ref = useRef<SVGSVGElement | any>(null);
  useEffect(() => {
    yScale && d3Select(ref.current).call(axisLeft<number>(yScale).tickFormat(yFormatter));
  }, [yScale]);
  return (
    <>
      <g className="y axis" ref={ref} />
      {label && (
        <text style={{fontSize: "0.6rem"}} transform="translate(0,-2)">
          {label}
        </text>
      )}
    </>
  );
};
