import "./style.scss";
import { Theme } from "../../../types";
import Toggle from "../../Main/Toggle/Toggle";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { FC } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";

const options = Object.values(Theme).map(value => ({ title: value, value }));
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
