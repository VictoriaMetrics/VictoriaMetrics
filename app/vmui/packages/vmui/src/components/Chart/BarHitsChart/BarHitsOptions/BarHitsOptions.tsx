import { FC, useEffect, useMemo, useRef } from "preact/compat";
import { GraphOptions, GRAPH_STYLES } from "../types";
import Switch from "../../../Main/Switch/Switch";
import "./style.scss";
import useStateSearchParams from "../../../../hooks/useStateSearchParams";
import { useSearchParams } from "react-router-dom";
import Button from "../../../Main/Button/Button";
import { SettingsIcon, VisibilityIcon, VisibilityOffIcon } from "../../../Main/Icons";
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

  const [stacked, setStacked] = useStateSearchParams(false, "stacked");
  const [fill, setFill] = useStateSearchParams("true", "fill");
  const [hideChart, setHideChart] = useStateSearchParams(false, "hide_chart");

  const options: GraphOptions = useMemo(() => ({
    graphStyle: GRAPH_STYLES.BAR,
    stacked,
    fill: fill === "true",
    hideChart,
  }), [stacked, fill, hideChart]);

  const handleChangeFill = (val: boolean) => {
    setFill(`${val}`);
    searchParams.set("fill", `${val}`);
    setSearchParams(searchParams);
  };

  const handleChangeStacked = (val: boolean) => {
    setStacked(val);
    val ? searchParams.set("stacked", "true") : searchParams.delete("stacked");
    setSearchParams(searchParams);
  };

  const toggleHideChart = () => {
    setHideChart(prev => {
      const newVal = !prev;
      newVal ? searchParams.set("hide_chart", "true") : searchParams.delete("hide_chart");
      setSearchParams(searchParams);
      return newVal;
    });
  };

  useEffect(() => {
    onChange(options);
  }, [options]);

  return (
    <div className="vm-bar-hits-options">
      <div ref={optionsButtonRef}>
        <Tooltip title="Graph settings">
          <Button
            variant="text"
            color="primary"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpenOptions}
            ariaLabel="settings"
          />
        </Tooltip>
      </div>
      <Tooltip title={hideChart ? "Show chart and resume hits updates" : "Hide chart and pause hits updates"}>
        <Button
          variant="text"
          color="primary"
          startIcon={hideChart ? <VisibilityOffIcon/> : <VisibilityIcon/>}
          onClick={toggleHideChart}
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
              value={fill === "true"}
              onChange={handleChangeFill}
            />
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default BarHitsOptions;
