import React from "react";
import "./style.scss";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { Theme } from "../../../types";
import Toggle from "../../Main/Toggle/Toggle";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";

const options = Object.values(Theme).map(value => ({ title: value, value }));
const ThemeControl = () => {
  const { isMobile } = useDeviceDetect();
  const { theme } = useAppState();
  const dispatch = useAppDispatch();

  const handleClickItem = (value: string) => {
    dispatch({ type: "SET_THEME", payload: value as Theme });
  };

  return (
    <div
      className={classNames({
        "vm-theme-control": true,
        "vm-theme-control_mobile": isMobile
      })}
    >
      <div className="vm-server-configurator__title">
        Theme preferences
      </div>
      <div
        className="vm-theme-control__toggle"
        key={`${isMobile}`}
      >
        <Toggle
          options={options}
          value={theme}
          onChange={handleClickItem}
        />
      </div>
    </div>
  );
};

export default ThemeControl;
