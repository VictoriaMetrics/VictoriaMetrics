import { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Tooltip from "../../Main/Tooltip/Tooltip";
import classNames from "classnames";
import { useNavigate } from "react-router-dom";
import {
  GroupIcon,
  AlertIcon,
  AlertingRuleIcon,
  RecordingRuleIcon,
  DetailsIcon,
  ArrowDownIcon,
} from "../../Main/Icons";
import Button from "../../Main/Button/Button";

interface ItemHeaderControlsProps {
  entity: string;
  type?: string;
  groupId: string;
  alertCount?: number;
  id?: string;
  name: string;
  onClose?: () => void;
}

const ItemHeader: FC<ItemHeaderControlsProps> = ({ name, id, groupId, entity, type, alertCount, onClose }) => {
  const { isMobile } = useDeviceDetect();
  const navigate = useNavigate();

  const openItemLink = () => {
    navigate({
      pathname: "/rules",
      search:   `group_id=${groupId}&${entity}_id=${id}`,
    });
  };

  const openGroupLink = () => {
    if (onClose) onClose();
    navigate({
      pathname: "/rules",
      hash: `#group-${groupId}`,
    });
  };

  const headerClasses = classNames({
    "vm-explore-alerts-item-header": true,
    "vm-explore-alerts-item-header_mobile": isMobile,
  });

  const renderIcon = () => {
    switch(entity) {
      case "alert":
        return (
          <Tooltip title="Alert">
            <AlertIcon />
          </Tooltip>
        );
      case "group":
        return (
          <Tooltip title="Group">
            <GroupIcon />
          </Tooltip>
        );
      default:
        switch(type) {
          case "alerting":
            return (
              <Tooltip title="Alerting rule">
                <AlertingRuleIcon />
              </Tooltip>
            );
          default:
            return (
              <Tooltip title="Recording rule">
                <RecordingRuleIcon />
              </Tooltip>
            );
        }
    }
  };

  return (
    <div
      className={headerClasses}
      id={`rule-${id}`}
    >
      <div className="vm-explore-alerts-item-header__title">
        {renderIcon()}
        <div className="vm-explore-alerts-item-header__name">{name}</div>
      </div>
      <div className="badge-container">
        {type === "alerting" && !!alertCount && (
          <div className="badge firing">firing: {alertCount}</div>
        )}
        {onClose ? (
          <Button
            className="vm-back-button"
            size="small"
            variant="outlined"
            color="gray"
            startIcon={<ArrowDownIcon />}
            onClick={openGroupLink}
          >
            <span className="vm-button-text">Rule Group</span>
          </Button>
        ) : (
          <Button
            className="vm-button-borderless"
            size="small"
            variant="outlined"
            color="gray"
            startIcon={<DetailsIcon />}
            onClick={openItemLink}
          />
        )}
      </div>
    </div>
  );
};

export default ItemHeader;
