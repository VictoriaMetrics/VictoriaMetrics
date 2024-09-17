import React, { FC, useCallback, useEffect, useState } from "preact/compat";
import uPlot, { Series } from "uplot";
import "./style.scss";
import "../../Line/Legend/style.scss";
import classNames from "classnames";
import { MouseEvent } from "react";
import { isMacOs } from "../../../../utils/detect-device";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { getStreamPairs } from "../../../../utils/logs";

interface Props {
  uPlotInst: uPlot;
  onApplyFilter: (value: string) => void;
}

const BarHitsLegend: FC<Props> = ({ uPlotInst, onApplyFilter }) => {
  const [series, setSeries] = useState<Series[]>([]);
  const [pairs, setPairs] = useState<string[][]>([]);

  const updateSeries = useCallback(() => {
    const series = uPlotInst.series.filter(s => s.scale !== "x");
    setSeries(series);
    setPairs(series.map(s => getStreamPairs(s.label || "")));
  }, [uPlotInst]);

  const handleClickByValue = (value: string) => (e: MouseEvent<HTMLDivElement>) => {
    const metaKey = e.metaKey || e.ctrlKey;
    if (!metaKey) return;
    onApplyFilter(`{${value}}` || "");
    updateSeries();
    uPlotInst.redraw();
  };

  const handleClickByStream = (target: Series) => (e: MouseEvent<HTMLDivElement>) => {
    const metaKey = e.metaKey || e.ctrlKey;
    if (metaKey) return;
    target.show = !target.show;
    updateSeries();
    uPlotInst.redraw();
  };

  useEffect(updateSeries, [uPlotInst]);

  return (
    <div className="vm-bar-hits-legend">
      {series.map((s, i) => (
        <Tooltip
          key={s.label}
          title={(
            <ul className="vm-bar-hits-legend-info">
              <li>Click to {s.show ? "hide" : "show"} the _stream.</li>
              <li>{isMacOs() ? "Cmd" : "Ctrl"} + Click to filter by the _stream.</li>
            </ul>
          )}
        >
          <div
            className={classNames({
              "vm-bar-hits-legend-item": true,
              "vm-bar-hits-legend-item_hide": !s.show,
            })}
            onClick={handleClickByStream(s)}
          >
            <div
              className="vm-bar-hits-legend-item__marker"
              style={{ backgroundColor: `${(s?.stroke as () => string)?.()}` }}
            />
            <div className="vm-bar-hits-legend-item-pairs">
              {pairs[i].map(value => (
                <span
                  className="vm-bar-hits-legend-item-pairs__value"
                  key={value}
                  onClick={handleClickByValue(value)}
                >
                  {value}
                </span>
              ))}
            </div>
          </div>
        </Tooltip>
      ))}
    </div>
  );
};

export default BarHitsLegend;
