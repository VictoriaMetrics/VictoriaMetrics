import React, { FC } from "preact/compat";
import "./style.scss";
import { LegendLogHits } from "../../../../api/types";
import LegendHitsMenuStats from "./LegendHitsMenuStats";
import LegendHitsMenuBase from "./LegendHitsMenuBase";
import LegendHitsMenuRow from "./LegendHitsMenuRow";
import LegendHitsMenuFields from "./LegendHitsMenuFields";
import { LOGS_LIMIT_HITS } from "../../../../constants/logs";

const otherDescription = `aggregated results for fields not in the top ${LOGS_LIMIT_HITS}`;

interface Props {
  legend: LegendLogHits;
  fields: string[];
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenu: FC<Props> = ({ legend, fields, onApplyFilter, onClose }) => {
  return (
    <div className="vm-legend-hits-menu">
      <div className="vm-legend-hits-menu-section">
        <LegendHitsMenuRow
          className="vm-legend-hits-menu-row_info"
          title={legend.isOther ? otherDescription : legend.label}
        />
      </div>

      {!legend.isOther && (
        <LegendHitsMenuBase
          legend={legend}
          onApplyFilter={onApplyFilter}
          onClose={onClose}
        />
      )}

      {!legend.isOther && (
        <LegendHitsMenuFields
          fields={fields}
          onApplyFilter={onApplyFilter}
          onClose={onClose}
        />
      )}

      <LegendHitsMenuStats legend={legend}/>
    </div>
  );
};

export default LegendHitsMenu;
