import React, { FC } from "preact/compat";
import "./style.scss";
import { InfoIcon } from "../../../Main/Icons";
import { LegendLogHits } from "../../../../api/types";
import LegendHitsMenuStats from "./LegendHitsMenuStats";
import LegendHitsMenuBase from "./LegendHitsMenuBase";
import LegendHitsMenuRow from "./LegendHitsMenuRow";
import LegendHitsMenuFields from "./LegendHitsMenuFields";
import LegendHitsMenuOther from "./LegendHitsMenuOther";

interface Props {
  legend: LegendLogHits;
  fields: string[];
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenu: FC<Props> = ({ legend, fields, onApplyFilter, onClose }) => {
  const hasIncludesHits = !!legend?.includesHits?.length;
  const titleForOther = <>Includes <b>{legend?.includesHits?.length || 0} hits</b> not in the top by total.</>;

  return (
    <div className="vm-legend-hits-menu">
      <div className="vm-legend-hits-menu-section">
        <LegendHitsMenuRow
          className="vm-legend-hits-menu-row_info"
          title={hasIncludesHits ? titleForOther : legend.label}
          iconStart={<InfoIcon/>}
        />
      </div>

      {!legend.isOther && (
        <LegendHitsMenuBase
          legend={legend}
          onApplyFilter={onApplyFilter}
          onClose={onClose}
        />
      )}

      {hasIncludesHits && (
        <div className="vm-legend-hits-menu-section">
          <LegendHitsMenuOther
            legend={legend}
            onApplyFilter={onApplyFilter}
            onClose={onClose}
          />
        </div>
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
