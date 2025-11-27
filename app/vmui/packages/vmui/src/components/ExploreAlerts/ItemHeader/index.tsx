import { FC, useMemo } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { useAppState } from "../../../state/common/StateContext";
import Tooltip from "../../Main/Tooltip/Tooltip";
import classNames from "classnames";
import { useNavigate } from "react-router-dom";
import Badges, { BadgeColor } from "../Badges";
import {
  LinkIcon,
  GroupIcon,
  AlertIcon,
  AlertingRuleIcon,
  RecordingRuleIcon,
  DetailsIcon,
} from "../../Main/Icons";
import Button from "../../Main/Button/Button";

interface ItemHeaderControlsProps {
  classes?: string[];
  entity: string;
  type?: string;
  groupId: string;
  states?: Record<string, number>;
  id?: string;
  name: string;
  onClose?: () => void;
}

const ItemHeader: FC<ItemHeaderControlsProps> = ({ name, id, groupId, entity, type, states, onClose, classes }) => {
  const { isMobile } = useDeviceDetect();
  const { serverUrl } = useAppState();
  const navigate = useNavigate();
  const copyToClipboard = useCopyToClipboard();

  const openGroupLink = () => {
    navigate({
      pathname: "/rules",
      search:   `group_id=${groupId}`,
    });
  };

  const openItemLink = () => {
    navigate({
      pathname: "/rules",
      search:   `group_id=${groupId}&${entity}_id=${id}`,
    });
  };

  const copyLink = async () => {
    let link = `${serverUrl}/vmui/#/rules?group_id=${groupId}`;
    if (type) link = `${link}&${entity}_id=${id}`;
    await copyToClipboard(link, `Link to ${entity} has been copied`);
  };

  const headerClasses = classNames({
    "vm-explore-alerts-item-header": true,
    "vm-explore-alerts-item-header_mobile": isMobile,
  }, classes);

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

  const badgesItems = useMemo(() => {
    return Object.fromEntries(Object.entries(states || {}).map(([name,  value]) => [name, {
      color: name.toLowerCase().replace(" ", "-") as BadgeColor,
      value: value == 1 ? 0 : value,
    }]));
  }, [states]);

  return (
    <div
      className={headerClasses}
      id={`rule-${id}`}
    >
      <div className="vm-explore-alerts-item-header__title">
        {renderIcon()}
        <div className="vm-explore-alerts-item-header__name">{name}</div>
      </div>
      <div className="vm-explore-alerts-controls">
        <Badges
          align="end"
          items={badgesItems}
        />
        {onClose ? (
          <>
            {id && (
              <Button
                className="vm-back-button"
                size="small"
                variant="outlined"
                color="gray"
                startIcon={<GroupIcon />}
                onClick={openGroupLink}
              >
                <span className="vm-button-text">Open Group</span>
              </Button>
            )}
            <Button
              className="vm-back-button"
              size="small"
              variant="outlined"
              color="gray"
              startIcon={<LinkIcon />}
              onClick={copyLink}
            >
              <span className="vm-button-text">Copy Link</span>
            </Button>
          </>
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
