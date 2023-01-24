import React from "react";
import "./style.scss";
import classNames from "classnames";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";

const options = [
  { title: "Light", value: false },
  { title: "Dark", value: true }
];

const ThemeControl = () => {
  const { darkTheme } = useAppState();
  const dispatch = useAppDispatch();

  const createHandlerClickItem = (value: boolean) => () => {
    dispatch({ type: "SET_DARK_THEME", payload: value });
  };

  return (
    <div className="vm-theme-control">
      <div className="vm-server-configurator__title">
        Theme preferences
      </div>
      <div className="vm-theme-control-options">
        <div
          className="vm-theme-control-options__highlight"
          style={{ left: darkTheme ? "50%" : 0 }}
        />
        {options.map(item => (
          <div
            className={classNames({
              "vm-theme-control-options__item": true,
              "vm-theme-control-options__item_active": item.value === darkTheme
            })}
            onClick={createHandlerClickItem(item.value)}
            key={item.title}
          >{item.title}</div>
        ))}
      </div>
    </div>
  );
};

export default ThemeControl;
