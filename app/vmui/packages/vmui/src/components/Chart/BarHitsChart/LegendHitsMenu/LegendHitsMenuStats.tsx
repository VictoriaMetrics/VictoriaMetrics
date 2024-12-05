import React, { FC } from "preact/compat";
import { LegendLogHits } from "../../../../api/types";

interface Props {
  legend: LegendLogHits;
}

const LegendHitsMenuStats: FC<Props> = ({ legend }) => {
  const totalFormatted = legend.total.toLocaleString("en-US");
  const percentage = Math.round((legend.total / legend.totalHits) * 100);

  return (
    <div className="vm-legend-hits-menu-section">
      <div className="vm-legend-hits-menu-row">
        <div className="vm-legend-hits-menu-row__title">
          Total: {totalFormatted} ({percentage}%)
        </div>
      </div>
    </div>
  );
};

export default LegendHitsMenuStats;
