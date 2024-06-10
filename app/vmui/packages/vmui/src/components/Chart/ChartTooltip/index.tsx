import React, { FC } from "preact/compat";
import ChartTooltip, { ChartTooltipProps } from "./ChartTooltip";
import "./style.scss";

interface LineTooltipHook {
  showTooltip: boolean;
  tooltipProps: ChartTooltipProps;
  stickyTooltips: ChartTooltipProps[];
  handleUnStick: (id: string) => void;
}

const ChartTooltipWrapper: FC<LineTooltipHook> = ({ showTooltip, tooltipProps, stickyTooltips, handleUnStick }) => (
  <>
    {showTooltip && tooltipProps && <ChartTooltip {...tooltipProps}/>}
    {stickyTooltips.map(t => (
      <ChartTooltip
        {...t}
        isSticky
        key={t.id}
        onClose={handleUnStick}
      />
    ))}
  </>
);

export default ChartTooltipWrapper;
