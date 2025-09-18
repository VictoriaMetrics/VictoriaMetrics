import { FC, useMemo } from "preact/compat";
import { useNavigate } from "react-router-dom";
import router from "../../router";
import { getAppModeEnable, getAppModeParams } from "../../utils/app-mode";
import { LogoAnomalyIcon, LogoIcon } from "../../components/Main/Icons";
import { getCssVariable } from "../../utils/theme";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../state/common/StateContext";
import HeaderNav from "./HeaderNav/HeaderNav";
import SidebarHeader from "./SidebarNav/SidebarHeader";
import HeaderControls, { ControlsProps } from "./HeaderControls/HeaderControls";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import useWindowSize from "../../hooks/useWindowSize";
import { ComponentType } from "react";
import { APP_TYPE, AppType } from "../../constants/appType";

export interface HeaderProps {
  controlsComponent: ComponentType<ControlsProps>
}
const Logo = () => {
  switch (APP_TYPE) {
    case AppType.vmanomaly:
      return <LogoAnomalyIcon/>;
    default:
      return <LogoIcon/>;
  }
};

const Header: FC<HeaderProps> = ({ controlsComponent }) => {
  const { isMobile } = useDeviceDetect();

  const windowSize = useWindowSize();
  const displaySidebar = useMemo(() => window.innerWidth < 1230, [windowSize]);

  const { isDarkTheme } = useAppState();
  const appModeEnable = getAppModeEnable();

  const primaryColor = useMemo(() => {
    const variable = isDarkTheme ? "color-background-block" : "color-primary";
    return getCssVariable(variable);
  }, [isDarkTheme]);

  const { background, color } = useMemo(() => {
    const { headerStyles: {
      background = appModeEnable ? "#FFF" : primaryColor,
      color = appModeEnable ? primaryColor : "#FFF",
    } = {} } = getAppModeParams();

    return { background, color };
  }, [primaryColor]);

  const navigate = useNavigate();

  const onClickLogo = () => {
    navigate({ pathname: router.home });
    window.location.reload();
  };

  return <header
    className={classNames({
      "vm-header": true,
      "vm-header_app": appModeEnable,
      "vm-header_dark": isDarkTheme,
      "vm-header_sidebar": displaySidebar,
      "vm-header_mobile": isMobile
    })}
    style={{ background, color }}
  >
    {displaySidebar ? (
      <SidebarHeader
        background={background}
        color={color}
      />
    ) : (
      <>
        {!appModeEnable && (
          <div
            className="vm-header-logo"
            onClick={onClickLogo}
            style={{ color }}
          >
            {<Logo/>}
          </div>
        )}
        <HeaderNav
          color={color}
          background={background}
        />
      </>
    )}
    {displaySidebar && (
      <div
        className={classNames({
          "vm-header-logo": true,
          "vm-header-logo_mobile": true,
        })}
        onClick={onClickLogo}
        style={{ color }}
      >
        {<Logo/>}
      </div>
    )}
    <HeaderControls
      controlsComponent={controlsComponent}
      displaySidebar={displaySidebar}
      isMobile={isMobile}
      closeModal={() => {}}
    />
  </header>;
};

export default Header;
