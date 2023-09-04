import React from "react";
import "./style.scss";
import { Theme } from "../../../types";
import Toggle from "../../Main/Toggle/Toggle";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { FC } from "preact/compat";

interface ThemeControlProps {
  theme: Theme;
  onChange: (val: Theme) => void
}

const options = Object.values(Theme).map(value => ({ title: value, value }));
const ThemeControl: FC<ThemeControlProps> = ({ theme, onChange }) => {
  const { isMobile } = useDeviceDetect();

  const handleClickItem = (value: string) => {
    onChange(value as Theme);
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
