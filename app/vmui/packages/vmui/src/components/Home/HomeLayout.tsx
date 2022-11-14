import Header from "../Header/Header";
import React, { FC } from "preact/compat";
import { Outlet } from "react-router-dom";
import "./style.scss";
import { getAppModeEnable } from "../../utils/app-mode";
import classNames from "classnames";

const HomeLayout: FC = () => {
  const appModeEnable = getAppModeEnable();

  return <section className="vm-container">
    <Header/>
    <div
      className={classNames({
        "vm-container-body": true,
        "vm-container-body_app": appModeEnable
      })}
    >
      <Outlet/>
    </div>
  </section>;
};

export default HomeLayout;
