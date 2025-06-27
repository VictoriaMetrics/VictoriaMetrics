import React, { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Group } from "../../../types";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { CopyIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";

interface GroupHeaderControlsProps {
  group: Group,
}

const GroupHeaderHeader: FC<GroupHeaderControlsProps> = ({
  group,
}) => {
  const { isMobile } = useDeviceDetect();
  const copyToClipboard = useCopyToClipboard();

  const handlerCopy = async () => {
    const link = `${window.location.origin}${window.location.pathname}#/alert-rules#group-${group.id}`;
    await copyToClipboard(link, "Link to group has been copied");
  };

  if (isMobile) {
    return (
      <div className="vm-explore-alerts-group-header vm-explore-alerts-group-header_mobile">
        <div className="vm-explore-alerts-group-header__name">{group.name}</div>
        <div className="circle-container">
          {Object.entries(group.states || {}).map(([name, value]) => (
            <div
              key={name}
              className={`circle ${name.toLowerCase().replace(" ", "-")}`}
            >{value}</div>
          ))}
          <Button
            variant="outlined"
            startIcon={<CopyIcon/>}
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
      <div className="circle-container">
        {Object.entries(group.states || {}).map(([name, value]) => (
          <div
            key={name}
            className={`circle ${name.toLowerCase().replace(" ", "-")}`}
          >{value}</div>
        ))}
        <Button
          variant="outlined"
          startIcon={<CopyIcon/>}
          onClick={handlerCopy}
        />
      </div>
    </div>
  );
};

export default GroupHeaderHeader;
