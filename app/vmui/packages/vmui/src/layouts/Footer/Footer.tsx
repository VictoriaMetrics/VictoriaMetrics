import React, { FC, memo } from "preact/compat";
import { CodeIcon, IssueIcon, LogoShortIcon, WikiIcon } from "../../components/Main/Icons";
import "./style.scss";

const Footer: FC = memo(() => {
  const copyrightYears = `2019-${new Date().getFullYear()}`;

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
      Documentation
    </a>
    <a
      className="vm-link vm-footer__link"
      target="_blank"
      href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose"
      rel="noreferrer"
    >
      <IssueIcon/>
      Create an issue
    </a>
    <div className="vm-footer__copyright">
      &copy; {copyrightYears} VictoriaMetrics
    </div>
  </footer>;
});

export default Footer;
