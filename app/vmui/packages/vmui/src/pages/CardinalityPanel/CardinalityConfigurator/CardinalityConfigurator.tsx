import React, { FC, useMemo } from "react";
import { PlayIcon, QuestionIcon, RestartIcon, TipIcon, WikiIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { useEffect } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import CardinalityTotals, { CardinalityTotalsProps } from "../CardinalityTotals/CardinalityTotals";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";

const CardinalityConfigurator: FC<CardinalityTotalsProps> = ({ isPrometheus, isCluster, ...props }) => {
  const { isMobile } = useDeviceDetect();
  const [searchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const showTips = searchParams.get("tips") || "";
  const [match, setMatch] = useStateSearchParams("", "match");
  const [focusLabel, setFocusLabel] = useStateSearchParams("", "focusLabel");
  const [topN, setTopN] = useStateSearchParams(10, "topN");

  const errorTopN = useMemo(() => topN < 0 ? "Number must be bigger than zero" : "", [topN]);

  const handleTopNChange = (val: string) => {
    const num = +val;
    setTopN(isNaN(num) ? 0 : num);
  };

  const handleRunQuery = () => {
    setSearchParamsFromKeys({ match, topN, focusLabel });
  };

  const handleResetQuery = () => {
    setSearchParamsFromKeys({ match: "", focusLabel: "" });
  };

  const handleToggleTips = () => {
    const showTips = searchParams.get("tips") || "";
    setSearchParamsFromKeys({ tips: showTips ? "" : "true" });
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
          value={isPrometheus ? 10 : topN}
          error={errorTopN}
          disabled={isPrometheus}
          helperText={isPrometheus ? "not available for Prometheus" : ""}
          onChange={handleTopNChange}
          onEnter={handleRunQuery}
        />
      </div>
    </div>
    <div className="vm-cardinality-configurator-bottom">
      <CardinalityTotals
        isPrometheus={isPrometheus}
        isCluster={isCluster}
        {...props}
      />
      {isCluster &&
        <div className="vm-cardinality-configurator-bottom-helpful">
          <Hyperlink
            href="https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cardinality-explorer-statistic-inaccuracy"
            withIcon={true}
          >
            <WikiIcon/>
          Statistic inaccuracy explanation
          </Hyperlink>
        </div>
      }

      <div className="vm-cardinality-configurator-bottom-helpful">
        <Hyperlink
          href="https://docs.victoriametrics.com/#cardinality-explorer"
          withIcon={true}
        >
          <WikiIcon/>
          Documentation
        </Hyperlink>
      </div>

      <div className="vm-cardinality-configurator-bottom__execute">
        <Tooltip title={showTips ? "Hide tips" : "Show tips"}>
          <Button
            variant="text"
            color={showTips ? "warning" : "gray"}
            startIcon={<TipIcon/>}
            onClick={handleToggleTips}
            ariaLabel="visibility tips"
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
