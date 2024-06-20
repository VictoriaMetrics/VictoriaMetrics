import Header from "../Header/Header";
import React, { FC, useEffect } from "preact/compat";
import { Outlet, useLocation } from "react-router-dom";
import "../MainLayout/style.scss";
import { getAppModeEnable } from "../../utils/app-mode";
import classNames from "classnames";
import Footer from "../Footer/Footer";
import router, { routerOptions } from "../../router";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import ControlsLogsLayout from "./ControlsLogsLayout";
import useFetchDefaultTimezone from "../../hooks/useFetchDefaultTimezone";
import { footerLinksToLogs } from "../../constants/footerLinks";

const LogsLayout: FC = () => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();
  const { pathname } = useLocation();

  useFetchDefaultTimezone();

  const setDocumentTitle = () => {
    const defaultTitle = "vmui for VictoriaLogs";
    const routeTitle = routerOptions[router.logs]?.title;
    document.title = routeTitle ? `${routeTitle} - ${defaultTitle}` : defaultTitle;
  };

  useEffect(setDocumentTitle, [pathname]);

  return <section className="vm-container">
    <Header controlsComponent={ControlsLogsLayout}/>
    <div
      className={classNames({
        "vm-container-body": true,
        "vm-container-body_mobile": isMobile,
        "vm-container-body_app": appModeEnable
      })}
    >
      <Outlet/>
    </div>
    {!appModeEnable && <Footer links={footerLinksToLogs}/>}
  </section>;
};

export default LogsLayout;
