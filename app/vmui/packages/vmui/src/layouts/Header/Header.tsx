import { FC, useMemo } from "preact/compat";
import { useNavigate } from "react-router-dom";
import router from "../../router";
import { getAppModeEnable, getAppModeParams } from "../../utils/app-mode";
import { LogoIcon } from "../../components/Main/Icons";
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

export interface HeaderProps {
  controlsComponent: ComponentType<ControlsProps>
}

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
    <div
      className={classNames({
        "vm-header-logo": true,
        "vm-header-logo_mobile": displaySidebar,
      })}
      onClick={onClickLogo}
      style={{ color }}
    >
      {<LogoIcon/>}
    </div>

    {displaySidebar ? (
      <SidebarHeader
        background={background}
        color={color}
      />
    ) : (
      <HeaderNav
        color={color}
        background={background}
      />
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
