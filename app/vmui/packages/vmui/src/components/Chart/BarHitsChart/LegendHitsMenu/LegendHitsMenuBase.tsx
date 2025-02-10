import React, { FC } from "preact/compat";
import LegendHitsMenuRow from "./LegendHitsMenuRow";
import useCopyToClipboard from "../../../../hooks/useCopyToClipboard";
import { CopyIcon, FilterIcon, FilterOffIcon } from "../../../Main/Icons";
import { LegendLogHits, LegendLogHitsMenu } from "../../../../api/types";
import { LOGS_GROUP_BY } from "../../../../constants/logs";

interface Props {
  legend: LegendLogHits;
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenuBase: FC<Props> = ({ legend, onApplyFilter, onClose }) => {
  const copyToClipboard = useCopyToClipboard();

  const handleAddStreamToFilter = () => {
    onApplyFilter(`${LOGS_GROUP_BY}: ${legend.label}`);
    onClose();
  };

  const handleExcludeStreamToFilter = () => {
    onApplyFilter(`(NOT ${LOGS_GROUP_BY}: ${legend.label})`);
    onClose();
  };

  const handlerCopyLabel = async () => {
    await copyToClipboard(legend.label, `${legend.label} has been copied`);
    onClose();
  };

  const options: LegendLogHitsMenu[] = [
    {
      title: `Copy ${LOGS_GROUP_BY} name`,
      icon: <CopyIcon/>,
      handler: handlerCopyLabel,
    },
    {
      title: `Add ${LOGS_GROUP_BY} to filter`,
      icon: <FilterIcon/>,
      handler: handleAddStreamToFilter,
    },
    {
      title: `Exclude ${LOGS_GROUP_BY} to filter`,
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
