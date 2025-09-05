import { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { useNavigate } from "react-router-dom";
import { Group as APIGroup } from "../../../types";
import { DetailsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Badges, { BadgeColor } from "../Badges";
import classNames from "classnames";
interface GroupHeaderControlsProps {
  group: APIGroup;
}

const GroupHeaderHeader: FC<GroupHeaderControlsProps> = ({ group }) => {
  const { isMobile } = useDeviceDetect();
  const navigate = useNavigate();

  const openGroupModal = async () => {
    navigate({
      pathname: "/rules",
      search:   `group_id=${group.id}`,
    });
  };

  const headerClasses = classNames({
    "vm-explore-alerts-group-header": true,
    "vm-explore-alerts-group-header_mobile": isMobile,
  });

  return (
    <div className={headerClasses}>
      <div className="vm-explore-alerts-group-header__desc">
        <div className="vm-explore-alerts-group-header__name">{group.name}</div>
        {!isMobile && (
          <div className="vm-explore-alerts-group-header__file">{group.file}</div>
        )}
      </div>
      <div className="vm-explore-alerts-controls">
        <Badges
          align="end"
          items={Object.fromEntries(Object.entries(group.states || {}).map(([name, value]) => [name.toLowerCase(), {
            color: name.toLowerCase().replace(" ", "-") as BadgeColor,
            value: value,
          }]))}
        />
        <Button
          className="vm-button-borderless"
          size="small"
          color="gray"
          variant="outlined"
          startIcon={<DetailsIcon />}
          onClick={openGroupModal}
        />
      </div>
    </div>
  );
};

export default GroupHeaderHeader;
