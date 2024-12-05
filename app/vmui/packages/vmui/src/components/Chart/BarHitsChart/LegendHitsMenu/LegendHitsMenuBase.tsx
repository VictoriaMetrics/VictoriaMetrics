import React, { FC } from "preact/compat";
import LegendHitsMenuRow from "./LegendHitsMenuRow";
import useCopyToClipboard from "../../../../hooks/useCopyToClipboard";
import { HITS_GROUP_FIELD } from "../../../../pages/ExploreLogs/hooks/useFetchLogHits";
import { CopyIcon, FilterIcon, FilterOffIcon } from "../../../Main/Icons";
import { LegendLogHits, LegendLogHitsMenu } from "../../../../api/types";

interface Props {
  legend: LegendLogHits;
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenuBase: FC<Props> = ({ legend, onApplyFilter, onClose }) => {
  const copyToClipboard = useCopyToClipboard();

  const handleAddStreamToFilter = () => {
    onApplyFilter(`${HITS_GROUP_FIELD}: ${legend.label}`);
    onClose();
  };

  const handleExcludeStreamToFilter = () => {
    onApplyFilter(`(NOT ${HITS_GROUP_FIELD}: ${legend.label})`);
    onClose();
  };

  const handlerCopyLabel = async () => {
    await copyToClipboard(legend.label, `${legend.label} has been copied`);
    onClose();
  };

  const options: LegendLogHitsMenu[] = [
    {
      title: `Copy ${HITS_GROUP_FIELD} name`,
      icon: <CopyIcon/>,
      handler: handlerCopyLabel,
    },
    {
      title: `Add ${HITS_GROUP_FIELD} to filter`,
      icon: <FilterIcon/>,
      handler: handleAddStreamToFilter,
    },
    {
      title: `Exclude ${HITS_GROUP_FIELD} to filter`,
      icon: <FilterOffIcon/>,
      handler: handleExcludeStreamToFilter,
    }
  ];

  return (
    <div className="vm-legend-hits-menu-section">
      {options.map(({ icon, title, handler }) => (
        <LegendHitsMenuRow
          key={title}
          iconStart={icon}
          title={title}
          handler={handler}
        />
      ))}
    </div>
  );
};

export default LegendHitsMenuBase;
