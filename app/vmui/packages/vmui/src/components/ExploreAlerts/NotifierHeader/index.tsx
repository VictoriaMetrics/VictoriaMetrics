import React, { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Notifier } from "../../../types";

interface NotifierHeaderControlsProps {
  notifier: Notifier
}

const NotifierHeaderHeader: FC<NotifierHeaderControlsProps> = ({
  notifier,
}) => {
  const { isMobile } = useDeviceDetect();

  if (isMobile) {
    return (
      <div className="vm-explore-alerts-notifier-header vm-explore-alerts-notifier-header_mobile">
        <div className="vm-explore-alerts-notifier-header__name">{notifier.kind}</div>
      </div>
    );
  }

  return (
    <div className="vm-explore-alerts-notifier-header">
      <div className="vm-explore-alerts-notifier-header__name">{notifier.kind}</div>
    </div>
  );
};

export default NotifierHeaderHeader;
