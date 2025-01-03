import React, { FC, memo, useCallback, useEffect, useState } from "preact/compat";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import Button from "../../../components/Main/Button/Button";
import { CopyIcon } from "../../../components/Main/Icons";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";

interface Props {
  field: string;
  value: string;
}

const GroupLogsFieldRow: FC<Props> = ({ field, value }) => {
  const copyToClipboard = useCopyToClipboard();
  const [copied, setCopied] = useState<boolean>(false);

  const handleCopy = useCallback(async () => {
    if (copied) return;
    try {
      await copyToClipboard(`${field}: "${value}"`);
      setCopied(true);
    } catch (e) {
      console.error(e);
    }
  }, [copied, copyToClipboard]);

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
              variant="text"
              color="gray"
              size="small"
              startIcon={<CopyIcon/>}
              onClick={handleCopy}
              ariaLabel="copy to clipboard"
            />
          </Tooltip>
        </div>
      </td>
      <td className="vm-group-logs-row-fields-item__key">{field}</td>
      <td className="vm-group-logs-row-fields-item__value">{value}</td>
    </tr>
  );
};

export default memo(GroupLogsFieldRow);
