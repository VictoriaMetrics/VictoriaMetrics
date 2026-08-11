import "./style.scss";
import { Theme } from "../../../types";
import Toggle from "../../Main/Toggle/Toggle";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { FC } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { DarkIcon, LightIcon, SystemIcon } from "../../Main/Icons";

const themeIcons = {
  [Theme.system]: <SystemIcon/>,
  [Theme.light]: <LightIcon/>,
  [Theme.dark]: <DarkIcon/>,
};

const options = Object.values(Theme).map(value => ({
  title: value,
  value,
  icon: themeIcons[value],
}));

const ThemeControl: FC = () => {
  const { isMobile } = useDeviceDetect();
  const dispatch = useAppDispatch();

  const { theme } = useAppState();

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
        Theme
      </div>
      <div
        className="vm-theme-control__toggle"
        key={`${isMobile}`}
      >
        <Toggle
          size="large"
          options={options}
          value={theme}
          onChange={handleClickItem}
        />
      </div>
    </div>
  );
};

export default ThemeControl;
