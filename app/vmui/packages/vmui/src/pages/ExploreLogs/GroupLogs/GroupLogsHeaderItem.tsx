import React, { FC, useEffect } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import classNames from "classnames";
import { MouseEvent, useState } from "react";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { useSearchParams } from "react-router-dom";
import { LOGS_GROUP_BY, LOGS_URL_PARAMS } from "../../../constants/logs";
import { convertToFieldFilter } from "../../../utils/logs";

interface Props {
  pair: string;
  isHide?: boolean;
}

const GroupLogsHeaderItem: FC<Props> = ({ pair, isHide }) => {
  const { isDarkTheme } = useAppState();
  const copyToClipboard = useCopyToClipboard();
  const [searchParams] = useSearchParams();

  const [copied, setCopied] = useState<string | null>(null);

  const groupBy = searchParams.get(LOGS_URL_PARAMS.GROUP_BY) || LOGS_GROUP_BY;

  const handleClickByPair = (value: string) => async (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    const copyValue = convertToFieldFilter(value, groupBy);
    const isCopied = await copyToClipboard(copyValue);
    if (isCopied) {
      setCopied(value);
    }
  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(null), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <Tooltip
      title={copied === pair ? "Copied" : "Copy to clipboard"}
      placement={"top-center"}
    >
      <div
        className={classNames({
          "vm-group-logs-section-keys__pair": true,
          "vm-group-logs-section-keys__pair_hide": isHide,
          "vm-group-logs-section-keys__pair_dark": isDarkTheme
        })}
        onClick={handleClickByPair(pair)}
      >
        {pair}
      </div>
    </Tooltip>
  );
};

export default GroupLogsHeaderItem;
