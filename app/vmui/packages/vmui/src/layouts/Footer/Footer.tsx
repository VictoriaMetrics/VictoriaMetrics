import React, { FC } from "preact/compat";
import dayjs from "dayjs";
import "./style.scss";
import { CodeIcon, IssueIcon, LogoShortIcon, WikiIcon } from "../../components/Main/Icons";
import useDeviceDetect from "../../hooks/useDeviceDetect";

const Footer: FC = () => {
  const { isMobile } = useDeviceDetect();
  const copyrightYears = `2019-${dayjs().format("YYYY")}`;

  return <footer className="vm-footer">
    <a
      className="vm-link vm-footer__website"
      target="_blank"
      href="https://victoriametrics.com/"
      rel="me noreferrer"
    >
      <LogoShortIcon/>
      victoriametrics.com
    </a>
    <a
      className="vm-link vm-footer__link"
      target="_blank"
      href="https://docs.victoriametrics.com/MetricsQL.html"
      rel="help noreferrer"
    >
      <CodeIcon/>
      MetricsQL
    </a>
    <a
      className="vm-link vm-footer__link"
      target="_blank"
      href="https://docs.victoriametrics.com/#vmui"
      rel="help noreferrer"
    >
      <WikiIcon/>
      {isMobile ? "Docs" : "Documentation"}
    </a>
    <a
      className="vm-link vm-footer__link"
      target="_blank"
      href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose"
      rel="noreferrer"
    >
      <IssueIcon/>
      {isMobile ? "New issue" : "Create an issue"}
    </a>
    <div className="vm-footer__copyright">
      &copy; {copyrightYears} VictoriaMetrics
    </div>
  </footer>;
};

export default Footer;
