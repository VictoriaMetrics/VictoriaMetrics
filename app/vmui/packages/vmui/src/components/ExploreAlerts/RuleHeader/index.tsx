import React, { FC } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { CopyIcon } from "../../../components/Main/Icons";
import { Rule } from "../../../types";
import Button from "../../../components/Main/Button/Button";

interface RuleHeaderControlsProps {
  rule: Rule
}

const RuleHeader: FC<RuleHeaderControlsProps> = ({
  rule,
}) => {
  const { isMobile } = useDeviceDetect();
  const copyToClipboard = useCopyToClipboard();

  const handlerCopy = async () => {
    const link = `${window.location.origin}${window.location.pathname}#/alert-rules#rule-${rule.id}`;
    await copyToClipboard(link, "Link to rule has been copied");
  };

  if (isMobile) {
    return (
      <div
        className="vm-explore-alerts-rule-header vm-explore-alerts-rule-header_mobile"
        id={`rule-${rule.id}`}
      >
        <div className="vm-explore-alerts-rule-header__name">{rule.name}</div>
        <Button
          variant="outlined"
          startIcon={<CopyIcon/>}
          onClick={handlerCopy}
        /> 
      </div>
    );
  }

  return (
    <div
      className="vm-explore-alerts-rule-header"
      id={`rule-${rule.id}`}
    >
      <div className="vm-explore-alerts-rule-header__name">{rule.name}</div>
      <Button
        variant="outlined"
        startIcon={<CopyIcon/>}
        onClick={handlerCopy}
      />
    </div>
  );
};

export default RuleHeader;
