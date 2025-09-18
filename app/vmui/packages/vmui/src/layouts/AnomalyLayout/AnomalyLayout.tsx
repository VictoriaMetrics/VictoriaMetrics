import Header from "../Header/Header";
import { FC, useEffect } from "preact/compat";
import { Outlet, useSearchParams } from "react-router-dom";
import qs from "qs";
import "../MainLayout/style.scss";
import { getAppModeEnable } from "../../utils/app-mode";
import classNames from "classnames";
import Footer from "../Footer/Footer";
import useFetchDefaultTimezone from "../../hooks/useFetchDefaultTimezone";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import ControlsAnomalyLayout from "./ControlsAnomalyLayout";

const AnomalyLayout: FC = () => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();

  useFetchDefaultTimezone();

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

  useEffect(redirectSearchToHashParams, []);

  return <section className="vm-container">
    <Header controlsComponent={ControlsAnomalyLayout}/>
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

export default AnomalyLayout;
