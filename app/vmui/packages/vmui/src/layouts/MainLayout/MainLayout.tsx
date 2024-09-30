import Header from "../Header/Header";
import React, { FC, useEffect } from "preact/compat";
import { Outlet, useLocation, useSearchParams } from "react-router-dom";
import qs from "qs";
import "./style.scss";
import { getAppModeEnable } from "../../utils/app-mode";
import classNames from "classnames";
import Footer from "../Footer/Footer";
import { routerOptions } from "../../router";
import { useFetchDashboards } from "../../pages/PredefinedPanels/hooks/useFetchDashboards";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import ControlsMainLayout from "./ControlsMainLayout";
import useFetchDefaultTimezone from "../../hooks/useFetchDefaultTimezone";
import useFetchFlags from "../../hooks/useFetchFlags";

const MainLayout: FC = () => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();
  const { pathname } = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();

  useFetchDashboards();
  useFetchDefaultTimezone();
  useFetchFlags();

  const setDocumentTitle = () => {
    const defaultTitle = "vmui";
    const routeTitle = routerOptions[pathname]?.title;
    document.title = routeTitle ? `${routeTitle} - ${defaultTitle}` : defaultTitle;
  };

  // for support old links with search params
  const redirectSearchToHashParams = () => {
    const { search, href } = window.location;
    if (search) {
      const query = qs.parse(search, { ignoreQueryPrefix: true });
      Object.entries(query).forEach(([key, value]) => searchParams.set(key, value as string));
      setSearchParams(searchParams);
      window.location.search = "";
    }
    const newHref = href.replace(/\/\?#\//, "/#/");
    if (newHref !== href) window.location.replace(newHref);
  };

  useEffect(setDocumentTitle, [pathname]);
  useEffect(redirectSearchToHashParams, []);

  return <section className="vm-container">
    <Header controlsComponent={ControlsMainLayout}/>
    <div
      className={classNames({
        "vm-container-body": true,
        "vm-container-body_mobile": isMobile,
        "vm-container-body_app": appModeEnable
      })}
    >
      <Outlet/>
    </div>
    {!appModeEnable && <Footer/>}
  </section>;
};

export default MainLayout;
