import { FC, memo, useCallback, useEffect, useState } from "preact/compat";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import Button from "../../../components/Main/Button/Button";
import { CopyIcon, StorageIcon, VisibilityIcon } from "../../../components/Main/Icons";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { useSearchParams } from "react-router-dom";
import { LOGS_GROUP_BY, LOGS_URL_PARAMS } from "../../../constants/logs";

interface Props {
  field: string;
  value: string;
  hideGroupButton?: boolean;
}

const GroupLogsFieldRow: FC<Props> = ({ field, value, hideGroupButton }) => {
  const copyToClipboard = useCopyToClipboard();
  const [searchParams, setSearchParams] = useSearchParams();

  const [copied, setCopied] = useState<boolean>(false);

  const groupBy = searchParams.get(LOGS_URL_PARAMS.GROUP_BY) || LOGS_GROUP_BY;
  const displayFieldsString = searchParams.get(LOGS_URL_PARAMS.DISPLAY_FIELDS) || "";
  const displayFields = displayFieldsString ? displayFieldsString.split(",") : [];

  const isSelectedField = displayFields.includes(field);
  const isGroupByField = groupBy === field;

  const handleCopy = useCallback(async () => {
    if (copied) return;
    try {
      await copyToClipboard(`${field}: "${value}"`);
      setCopied(true);
    } catch (e) {
      console.error(e);
    }
  }, [copied, copyToClipboard]);

  const handleSelectDisplayField = () => {
    const prev = displayFields;
    const newDisplayFields = prev.includes(field) ? prev.filter(v => v !== field) : [...prev, field];
    searchParams.set(LOGS_URL_PARAMS.DISPLAY_FIELDS, newDisplayFields.join(","));
    setSearchParams(searchParams);
  };

  const handleSelectGroupBy = () => {
    isGroupByField ? searchParams.delete(LOGS_URL_PARAMS.GROUP_BY) : searchParams.set(LOGS_URL_PARAMS.GROUP_BY, field);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <tr className="vm-group-logs-row-fields-item">
      <td className="vm-group-logs-row-fields-item-controls">
        <div className="vm-group-logs-row-fields-item-controls__wrapper">
          <Tooltip title={copied ? "Copied" : "Copy to clipboard"}>
            <Button
              className="vm-group-logs-row-fields-item-controls__button"
              variant="text"
              color="gray"
              size="small"
              startIcon={<CopyIcon/>}
              onClick={handleCopy}
              ariaLabel="copy to clipboard"
            />
          </Tooltip>
          <Tooltip title={isSelectedField ? "Hide this field" : "Show this field instead of the message"}>
            <Button
              className="vm-group-logs-row-fields-item-controls__button"
              variant="text"
              color={isSelectedField ? "secondary" : "gray"}
              size="small"
              startIcon={isSelectedField ? <VisibilityIcon/> : <VisibilityIcon/>}
              onClick={handleSelectDisplayField}
              ariaLabel={isSelectedField ? "Hide this field" : "Show this field instead of the message"}
            />
          </Tooltip>
          {!hideGroupButton && (
            <Tooltip title={isGroupByField ? "Ungroup this field" : "Group by this field"}>
              <Button
                className="vm-group-logs-row-fields-item-controls__button"
                variant="text"
                color={isGroupByField ? "secondary" : "gray"}
                size="small"
                startIcon={<StorageIcon/>}
                onClick={handleSelectGroupBy}
                ariaLabel={isGroupByField ? "Ungroup this field" : "Group by this field"}
              />
            </Tooltip>
          )}
        </div>
      </td>
      <td className="vm-group-logs-row-fields-item__key">{field}</td>
      <td className="vm-group-logs-row-fields-item__value">{value}</td>
    </tr>
  );
};

export default memo(GroupLogsFieldRow);
