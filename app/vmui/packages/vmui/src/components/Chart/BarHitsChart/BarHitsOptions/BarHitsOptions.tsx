import React, { FC, useEffect, useMemo, useRef } from "preact/compat";
import { GraphOptions, GRAPH_STYLES } from "../types";
import Switch from "../../../Main/Switch/Switch";
import "./style.scss";
import useStateSearchParams from "../../../../hooks/useStateSearchParams";
import { useSearchParams } from "react-router-dom";
import Button from "../../../Main/Button/Button";
import classNames from "classnames";
import { SettingsIcon } from "../../../Main/Icons";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import Popper from "../../../Main/Popper/Popper";
import useBoolean from "../../../../hooks/useBoolean";

interface Props {
  onChange: (options: GraphOptions) => void;
}

const BarHitsOptions: FC<Props> = ({ onChange }) => {
  const [searchParams, setSearchParams] = useSearchParams();
  const optionsButtonRef = useRef<HTMLDivElement>(null);
  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

  const [graphStyle, setGraphStyle] = useStateSearchParams(GRAPH_STYLES.LINE_STEPPED, "graph");
  const [stacked, setStacked] = useStateSearchParams(false, "stacked");
  const [fill, setFill] = useStateSearchParams(false, "fill");

  const options: GraphOptions = useMemo(() => ({
    graphStyle,
    stacked,
    fill,
  }), [graphStyle, stacked, fill]);

  const handleChangeGraphStyle = (val: string) => () => {
    setGraphStyle(val as GRAPH_STYLES);
    searchParams.set("graph", val);
    setSearchParams(searchParams);
  };

  const handleChangeFill = (val: boolean) => {
    setFill(val);
    val ? searchParams.set("fill", "true") : searchParams.delete("fill");
    setSearchParams(searchParams);
  };

  const handleChangeStacked = (val: boolean) => {
    setStacked(val);
    val ? searchParams.set("stacked", "true") : searchParams.delete("stacked");
    setSearchParams(searchParams);
  };

  useEffect(() => {
    onChange(options);
  }, [options]);

  return (
    <div
      className="vm-bar-hits-options"
      ref={optionsButtonRef}
    >
      <Tooltip title="Graph settings">
        <Button
          variant="text"
          color="primary"
          startIcon={<SettingsIcon/>}
          onClick={toggleOpenOptions}
          ariaLabel="settings"
        />
      </Tooltip>
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={optionsButtonRef}
        title={"Graph settings"}
      >
        <div className="vm-bar-hits-options-settings">
          <div className="vm-bar-hits-options-settings-item vm-bar-hits-options-settings-item_list">
            <p className="vm-bar-hits-options-settings-item__title">Graph style:</p>
            {Object.values(GRAPH_STYLES).map(style => (
              <div
                key={style}
                className={classNames({
                  "vm-list-item": true,
                  "vm-list-item_active": graphStyle === style,
                })}
                onClick={handleChangeGraphStyle(style)}
              >
                {style}
              </div>
            ))}
          </div>
          <div className="vm-bar-hits-options-settings-item">
            <Switch
              label={"Stacked"}
              value={stacked}
              onChange={handleChangeStacked}
            />
          </div>
          <div className="vm-bar-hits-options-settings-item">
            <Switch
              label={"Fill"}
              value={fill}
              onChange={handleChangeFill}
            />
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default BarHitsOptions;
