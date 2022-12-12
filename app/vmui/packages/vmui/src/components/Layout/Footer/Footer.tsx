import React, { FC } from "preact/compat";
import dayjs from "dayjs";
import "./style.scss";
import { LogoIcon } from "../../Main/Icons";

const Footer: FC = () => {
  const copyrightYears = `2019-${dayjs().format("YYYY")}`;

  return <footer className="vm-footer">
    <a
      className="vm-footer__link vm-footer__website"
      target="_blank"
      href="https://victoriametrics.com/"
      rel="noreferrer"
    >
      <LogoIcon/>
      victoriametrics.com
    </a>
    <a
      className="vm-footer__link"
      target="_blank"
      href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new"
      rel="noreferrer"
    >
      create an issue
    </a>
    <div className="vm-footer__copyright">
      &copy; {copyrightYears} VictoriaMetrics
    </div>
  </footer>;
};

export default Footer;
