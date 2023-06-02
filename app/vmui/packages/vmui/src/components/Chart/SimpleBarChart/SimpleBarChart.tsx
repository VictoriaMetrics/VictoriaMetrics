import React, { FC, useEffect, useState } from "preact/compat";
import Tooltip from "../../Main/Tooltip/Tooltip";
import "./style.scss";

type BarChartData = {
  value: number,
  name: string,
  percentage?: number,
}

interface SimpleBarChartProps {
  data: BarChartData[],
}

const SimpleBarChart: FC<SimpleBarChartProps> = ({ data }) => {

  const [bars, setBars] = useState<BarChartData[]>([]);
  const [yAxis, setYAxis] = useState([0, 0]);

  const generateYAxis = (sortedValues: BarChartData[]) => {
    const numbers = sortedValues.map(b => b.value);
    const max = Math.ceil(numbers[0] || 1);
    const ticks = 10;
    const step = max / (ticks - 1);
    return new Array(ticks + 1).fill(max + step).map((v, i) => Math.round(v - (step * i)));
  };

  useEffect(() => {
    const sortedValues = data.sort((a, b) => b.value - a.value);
    const yAxis = generateYAxis(sortedValues);
    setYAxis(yAxis);

    setBars(sortedValues.map(b => ({
      ...b,
      percentage: (b.value / yAxis[0]) * 100,
    })));
  }, [data]);

  return (
    <div className="vm-simple-bar-chart">
      <div className="vm-simple-bar-chart-y-axis">
        {yAxis.map(v => (
          <div
            className="vm-simple-bar-chart-y-axis__tick"
            key={v}
          >{v}</div>
        ))}
      </div>
      <div className="vm-simple-bar-chart-data">
        {bars.map(({ name, value, percentage }) => (
          <Tooltip
            title={`${name}: ${value}`}
            key={`${name}_${value}`}
            placement="top-center"
          >
            <div
              className="vm-simple-bar-chart-data-item"
              style={{ maxHeight: `${percentage || 0}%` }}
            />
          </Tooltip>
        ))}
      </div>
    </div>
  );
};

export default SimpleBarChart;
