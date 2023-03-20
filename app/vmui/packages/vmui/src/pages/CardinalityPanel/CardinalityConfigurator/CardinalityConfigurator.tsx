import React, { FC, useMemo } from "react";
import { PlayIcon, QuestionIcon, RestartIcon, TipIcon, WikiIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import CardinalityTotals, { CardinalityTotalsProps } from "../CardinalityTotals/CardinalityTotals";

const CardinalityConfigurator: FC<CardinalityTotalsProps> = (props) => {
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();

  const showTips = searchParams.get("tips") || "";
  const [match, setMatch] = useState(searchParams.get("match") || "");
  const [focusLabel, setFocusLabel] = useState(searchParams.get("focusLabel") || "");
  const [topN, setTopN] = useState(+(searchParams.get("topN") || 10));

  const errorTopN = useMemo(() => topN < 0 ? "Number must be bigger than zero" : "", [topN]);

  const handleTopNChange = (val: string) => {
    const num = +val;
    setTopN(isNaN(num) ? 0 : num);
  };

  const handleRunQuery = () => {
    searchParams.set("match", match);
    searchParams.set("topN", topN.toString());
    searchParams.set("focusLabel", focusLabel);
    setSearchParams(searchParams);
  };

  const handleResetQuery = () => {
    searchParams.set("match", "");
    searchParams.set("focusLabel", "");
    setSearchParams(searchParams);
  };

  const handleToggleTips = () => {
    const showTips = searchParams.get("tips") || "";
    if (showTips) searchParams.delete("tips");
    else searchParams.set("tips", "true");
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const matchQuery = searchParams.get("match");
    const topNQuery = +(searchParams.get("topN") || 10);
    const focusLabelQuery = searchParams.get("focusLabel");
    if (matchQuery !== match) setMatch(matchQuery || "");
    if (topNQuery !== topN) setTopN(topNQuery);
    if (focusLabelQuery !== focusLabel) setFocusLabel(focusLabelQuery || "");
  }, [searchParams]);

  return <div
    className={classNames({
      "vm-cardinality-configurator": true,
      "vm-cardinality-configurator_mobile": isMobile,
      "vm-block": true,
      "vm-block_mobile": isMobile,
    })}
  >
    <div className="vm-cardinality-configurator-controls">
      <div className="vm-cardinality-configurator-controls__query">
        <TextField
          label="Time series selector"
          type="string"
          value={match}
          onChange={setMatch}
          onEnter={handleRunQuery}
        />
      </div>
      <div className="vm-cardinality-configurator-controls__item">
        <TextField
          label="Focus label"
          type="text"
          value={focusLabel || ""}
          onChange={setFocusLabel}
          onEnter={handleRunQuery}
          endIcon={(
            <Tooltip
              title={(
                <div>
                  <p>To identify values with the highest number of series for the selected label.</p>
                </div>
              )}
            >
              <QuestionIcon/>
            </Tooltip>
          )}
        />
      </div>
      <div className="vm-cardinality-configurator-controls__item vm-cardinality-configurator-controls__item_limit">
        <TextField
          label="Limit entries"
          type="number"
          value={topN}
          error={errorTopN}
          onChange={handleTopNChange}
          onEnter={handleRunQuery}
        />
      </div>
    </div>
    <div className="vm-cardinality-configurator-bottom">
      <CardinalityTotals {...props}/>

      <div className="vm-cardinality-configurator-bottom-helpful">
        <a
          className="vm-link vm-link_with-icon"
          target="_blank"
          href="https://docs.victoriametrics.com/#cardinality-explorer"
          rel="help noreferrer"
        >
          <WikiIcon/>
          Documentation
        </a>
      </div>

      <div className="vm-cardinality-configurator-bottom__execute">
        <Tooltip title={showTips ? "Hide tips" : "Show tips"}>
          <Button
            variant="text"
            color={showTips ? "warning" : "gray"}
            startIcon={<TipIcon/>}
            onClick={handleToggleTips}
          />
        </Tooltip>
        <Button
          variant="text"
          startIcon={<RestartIcon/>}
          onClick={handleResetQuery}
        >
          Reset
        </Button>
        <Button
          startIcon={<PlayIcon/>}
          onClick={handleRunQuery}
        >
          Execute Query
        </Button>
      </div>
    </div>
  </div>;
};

export default CardinalityConfigurator;
