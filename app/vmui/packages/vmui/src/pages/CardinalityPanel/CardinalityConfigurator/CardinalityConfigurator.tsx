import React, { FC, useMemo } from "react";
import { InfoIcon, PlayIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";

const CardinalityConfigurator: FC = () => {
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();

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
          label="Number of entries per table"
          type="number"
          value={topN}
          error={errorTopN}
          onChange={handleTopNChange}
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
                  <p>Adds a table showing the series with the highest number of series.</p>
                </div>
              )}
            >
              <InfoIcon/>
            </Tooltip>
          )}
        />
      </div>

      <Button
        startIcon={<PlayIcon/>}
        onClick={handleRunQuery}
        fullWidth
      >
        Execute Query
      </Button>
    </div>
  </div>;
};

export default CardinalityConfigurator;
