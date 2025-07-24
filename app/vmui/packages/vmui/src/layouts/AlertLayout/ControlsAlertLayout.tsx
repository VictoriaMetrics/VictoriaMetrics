import { FC } from "preact/compat";
import classNames from "classnames";
import GlobalSettings from "../../components/Configurators/GlobalSettings/GlobalSettings";
import { ControlsProps } from "../Header/HeaderControls/HeaderControls";

const ControlsAlertLayout: FC<ControlsProps> = ({ isMobile }) => {

  return (
    <div
      className={classNames({
        "vm-header-controls": true,
        "vm-header-controls_mobile": isMobile,
      })}
    >
      <GlobalSettings/>
    </div>
  );
};

export default ControlsAlertLayout;
