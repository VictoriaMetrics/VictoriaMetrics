import { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Group as APIGroup } from "../../../types";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { LinkIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";

interface GroupHeaderControlsProps {
  group: APIGroup;
}

const GroupHeaderHeader: FC<GroupHeaderControlsProps> = ({ group }) => {
  const { isMobile } = useDeviceDetect();
  const copyToClipboard = useCopyToClipboard();

  const handlerCopy = async () => {
    const link = `${window.location.origin}${window.location.pathname}#/rules#group-${group.id}`;
    await copyToClipboard(link, "Link to group has been copied");
  };

  if (isMobile) {
    return (
      <div className="vm-explore-alerts-group-header vm-explore-alerts-group-header_mobile">
        <div className="vm-explore-alerts-group-header__name">{group.name}</div>
        <div className="badge-container">
          {Object.entries(group.states || {}).map(([name, value]) => (
            <div
              key={name}
              className={`badge ${name.toLowerCase().replace(" ", "-")}`}
            >
              {name.toLowerCase()}: {value}
            </div>
          ))}
          <Button
            color="gray"
            variant="outlined"
            startIcon={<LinkIcon />}
            onClick={handlerCopy}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="vm-explore-alerts-group-header">
      <div className="vm-explore-alerts-group-header__desc">
        <div className="vm-explore-alerts-group-header__name">{group.name}</div>
        <div className="vm-explore-alerts-group-header__file">{group.file}</div>
      </div>
      <div className="badge-container">
        {Object.entries(group.states || {}).map(([name, value]) => (
          <div
            key={name}
            className={`badge ${name.toLowerCase().replace(" ", "-")}`}
          >
            {name.toLowerCase()}: {value}
          </div>
        ))}
        <Tooltip title="Copy group link">
          <Button
            color="gray"
            variant="outlined"
            startIcon={<LinkIcon />}
            onClick={handlerCopy}
          />
        </Tooltip>
      </div>
    </div>
  );
};

export default GroupHeaderHeader;
