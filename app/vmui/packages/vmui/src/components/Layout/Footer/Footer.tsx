import React, { FC } from "preact/compat";
import dayjs from "dayjs";
import "./style.scss";

const Footer: FC = () => {
  const copyrightYears = `2019-${dayjs().format("YYYY")}`;

  return <footer className="vm-footer">
    <div className="vm-footer__copyright">
      &copy; {copyrightYears} VictoriaMetrics
    </div>
  </footer>;
};

export default Footer;
