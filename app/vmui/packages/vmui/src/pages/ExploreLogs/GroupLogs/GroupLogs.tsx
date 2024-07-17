import React, { FC, useEffect, useMemo } from "preact/compat";
import { MouseEvent, useState } from "react";
import "./style.scss";
import { Logs } from "../../../api/types";
import Accordion from "../../../components/Main/Accordion/Accordion";
import { groupByMultipleKeys } from "../../../utils/array";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import GroupLogsItem from "./GroupLogsItem";
import { useAppState } from "../../../state/common/StateContext";
import classNames from "classnames";

interface TableLogsProps {
  logs: Logs[];
  columns: string[];
  markdownParsing: boolean;
}

const GroupLogs: FC<TableLogsProps> = ({ logs, markdownParsing }) => {
  const { isDarkTheme } = useAppState();
  const copyToClipboard = useCopyToClipboard();

  const [copied, setCopied] = useState<string | null>(null);

  const groupData = useMemo(() => {
    return groupByMultipleKeys(logs, ["_stream"]).map((item) => {
      const streamValue = item.values[0]?._stream || "";
      const pairs = streamValue.slice(1, -1).match(/(?:[^\\,]+|\\,)+?(?=,|$)/g) || [streamValue];
      return {
        ...item,
        pairs: pairs.filter(Boolean),
      };
    });
  }, [logs]);

  const handleClickByPair = (pair: string) => async (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    const isCopied = await copyToClipboard(`${pair.replace(/=/, ": ")}`);
    if (isCopied) {
      setCopied(pair);
    }
  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(null), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <div className="vm-group-logs">
      {groupData.map((item) => (
        <div
          className="vm-group-logs-section"
          key={item.keys.join("")}
        >
          <Accordion
            defaultExpanded={true}
            title={(
              <div className="vm-group-logs-section-keys">
                <span className="vm-group-logs-section-keys__title">Group by _stream:</span>
                {item.pairs.map((pair) => (
                  <Tooltip
                    title={copied === pair ? "Copied" : "Copy to clipboard"}
                    key={`${item.keys.join("")}_${pair}`}
                    placement={"top-center"}
                  >
                    <div
                      className={classNames({
                        "vm-group-logs-section-keys__pair": true,
                        "vm-group-logs-section-keys__pair_dark": isDarkTheme
                      })}
                      onClick={handleClickByPair(pair)}
                    >
                      {pair}
                    </div>
                  </Tooltip>
                ))}
              </div>
            )}
          >
            <div className="vm-group-logs-section-rows">
              {item.values.map((value) => (
                <GroupLogsItem
                  key={`${value._msg}${value._time}`}
                  log={value}
                  markdownParsing={markdownParsing}
                />
              ))}
            </div>
          </Accordion>
        </div>
      ))}
    </div>
  );
};

export default GroupLogs;
