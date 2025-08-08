import { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Notifier } from "../../../types";
import classNames from "classnames";

interface NotifierHeaderControlsProps {
  notifier: Notifier;
}

const NotifierHeaderHeader: FC<NotifierHeaderControlsProps> = ({
  notifier,
}) => {
  const { isMobile } = useDeviceDetect();

  return (
    <div
      className={classNames({
        "vm-explore-alerts-notifier-header": true,
        "vm-explore-alerts-notifier-header_mobile": isMobile,
      })}
    >
      <div className="vm-explore-alerts-notifier-header__name">
        {notifier.kind}
      </div>
    </div>
  );
};

export default NotifierHeaderHeader;
