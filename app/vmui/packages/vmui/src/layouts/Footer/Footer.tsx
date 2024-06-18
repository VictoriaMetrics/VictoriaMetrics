import React, { FC, memo } from "preact/compat";
import { LogoShortIcon } from "../../components/Main/Icons";
import "./style.scss";
import { footerLinksByDefault } from "../../constants/footerLinks";

interface Props {
  links?: {
    href: string;
    Icon: FC;
    title: string;
  }[]
}

const Footer: FC<Props> = memo(({ links = footerLinksByDefault }) => {
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
    {links.map(({ href, Icon, title }) => (
      <a
        className="vm-link vm-footer__link"
        target="_blank"
        href={href}
        rel="help noreferrer"
        key={`${href}-${title}`}
      >
        <Icon/>
        {title}
      </a>
    ))}
    <div className="vm-footer__copyright">
      &copy; {copyrightYears} VictoriaMetrics
    </div>
  </footer>;
});

export default Footer;
