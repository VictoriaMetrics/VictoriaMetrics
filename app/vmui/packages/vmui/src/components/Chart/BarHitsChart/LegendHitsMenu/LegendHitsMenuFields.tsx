import React, { FC, useMemo } from "preact/compat";
import LegendHitsMenuRow from "./LegendHitsMenuRow";
import { CopyIcon, FilterIcon, FilterOffIcon } from "../../../Main/Icons";
import { convertToFieldFilter } from "../../../../utils/logs";
import { LegendLogHitsMenu } from "../../../../api/types";
import useCopyToClipboard from "../../../../hooks/useCopyToClipboard";

interface Props {
  fields: string[];
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenuFields: FC<Props> = ({ fields, onApplyFilter, onClose }) => {
  const copyToClipboard = useCopyToClipboard();

  const handleCopy = (field: string) => async () => {
    await copyToClipboard(field, `${field} has been copied`);
    onClose();
  };

  const handleAddToFilter = (field: string) => () => {
    onApplyFilter(field);
    onClose();
  };

  const handleExcludeToFilter = (field: string) => () => {
    onApplyFilter(`-${field}`);
    onClose();
  };

  const generateFieldMenu = (field: string): LegendLogHitsMenu[] => {
    return [
      {
        title: "Copy",
        icon: <CopyIcon/>,
        handler: handleCopy(field),
      },
      {
        title: "Add to filter",
        icon: <FilterIcon/>,
        handler: handleAddToFilter(field),
      },
      {
        title: "Exclude to filter",
        icon: <FilterOffIcon/>,
        handler: handleExcludeToFilter(field),
      }
    ];
  };

  const fieldsWithMenu: LegendLogHitsMenu[] = useMemo(() => {
    return fields.map(field => {
      const title = convertToFieldFilter(field);
      return {
        title,
        submenu: generateFieldMenu(title),
      };
    });
  }, [fields]);

  return (
    <div className="vm-legend-hits-menu-section">
      {fieldsWithMenu?.map((field) => (
        <LegendHitsMenuRow
          key={field.title}
          {...field}
        />
      ))}
    </div>
  );
};

export default LegendHitsMenuFields;
