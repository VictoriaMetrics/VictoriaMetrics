import React, { FC, useMemo } from "preact/compat";
import { useNavigate } from "react-router-dom";
import router from "../../../router";
import { getAppModeEnable, getAppModeParams } from "../../../utils/app-mode";
import { LogoFullIcon } from "../../Main/Icons";
import { getCssVariable } from "../../../utils/theme";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../../state/common/StateContext";
import HeaderNav from "./HeaderNav/HeaderNav";
import useResize from "../../../hooks/useResize";
import SidebarHeader from "./SidebarNav/SidebarHeader";
import HeaderControls from "./HeaderControls/HeaderControls";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

const Header: FC = () => {
  const { isMobile } = useDeviceDetect();

  const windowSize = useResize(document.body);
  const displaySidebar = useMemo(() => window.innerWidth < 1000, [windowSize]);

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
            <LogoFullIcon/>
          </div>
        )}
        <HeaderNav
          color={color}
          background={background}
        />
      </>
    )}
    {isMobile && (
      <div
        className="vm-header-logo vm-header-logo_mobile"
        onClick={onClickLogo}
        style={{ color }}
      >
        <LogoFullIcon/>
      </div>
    )}
    <HeaderControls
      displaySidebar={displaySidebar}
      isMobile={isMobile}
    />
  </header>;
};

export default Header;
