import { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import Tooltip from "../../Main/Tooltip/Tooltip";
import {
  AlertingRuleIcon,
  RecordingRuleIcon,
  LinkIcon,
} from "../../Main/Icons";
import { Rule } from "../../../types";
import Button from "../../Main/Button/Button";

interface RuleHeaderControlsProps {
  rule: Rule;
}

const RuleHeader: FC<RuleHeaderControlsProps> = ({ rule }) => {
  const { isMobile } = useDeviceDetect();
  const copyToClipboard = useCopyToClipboard();

  const handlerCopy = async () => {
    const link = `${window.location.origin}${window.location.pathname}#/rules#rule-${rule.id}`;
    await copyToClipboard(link, "Link to rule has been copied");
  };

  return (
    <div
      className={`vm-explore-alerts-rule-header ${isMobile && "vm-explore-alerts-rule-header_mobile"}`}
      id={`rule-${rule.id}`}
    >
      <div className="vm-explore-alerts-rule-header__title">
        {rule.type === "alerting" ? (
          <Tooltip title="Alerting rule">
            <AlertingRuleIcon />
          </Tooltip>
        ) : (
          <Tooltip title="Recording rule">
            <RecordingRuleIcon />
          </Tooltip>
        )}
        <div className="vm-explore-alerts-rule-header__name">{rule.name}</div>
      </div>
      <div className="badge-container">
        {rule.type === "alerting" && rule.alerts?.length && (
          <div className="badge firing">firing: {rule.alerts.length}</div>
        )}
        {isMobile ? (
          <Button
            variant="outlined"
            color="gray"
            startIcon={<LinkIcon />}
            onClick={handlerCopy}
          />
        ) : (
          <Tooltip title="Copy rule link">
            <Button
              color="gray"
              variant="outlined"
              startIcon={<LinkIcon />}
              onClick={handlerCopy}
            />
          </Tooltip>
        )}
      </div>
    </div>
  );
};

export default RuleHeader;
