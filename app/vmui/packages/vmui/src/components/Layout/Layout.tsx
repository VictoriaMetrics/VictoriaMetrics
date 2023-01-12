import Header from "./Header/Header";
import React, { FC, useEffect } from "preact/compat";
import { Outlet, useLocation } from "react-router-dom";
import "./style.scss";
import { getAppModeEnable } from "../../utils/app-mode";
import classNames from "classnames";
import Footer from "./Footer/Footer";
import { routerOptions } from "../../router";
import { useFetchDashboards } from "../../pages/PredefinedPanels/hooks/useFetchDashboards";

const Layout: FC = () => {
  const appModeEnable = getAppModeEnable();
  useFetchDashboards();

  const { pathname } = useLocation();
  useEffect(() => {
    const defaultTitle = "VM UI";
    const routeTitle = routerOptions[pathname]?.title;
    document.title = routeTitle ? `${routeTitle} - ${defaultTitle}` : defaultTitle;
  }, [pathname]);

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
    {!appModeEnable && <Footer/>}
  </section>;
};

export default Layout;
