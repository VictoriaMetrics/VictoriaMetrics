import { FC, memo } from "preact/compat";
import { LogoShortIcon } from "../../components/Main/Icons";
import "./style.scss";
import { footerLinksByDefault } from "../../constants/footerLinks";
import { useAppState } from "../../state/common/StateContext";

interface Props {
  links?: {
    href: string;
    Icon: FC;
    title: string;
  }[]
}

const Footer: FC<Props> = memo(({ links = footerLinksByDefault }) => {
  const copyrightYears = `2019-${new Date().getFullYear()}`;
  const { appConfig } = useAppState();
  const version = appConfig?.version;

  return <footer className="vm-footer">
    <a
      className="vm-link vm-footer__link"
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
      &copy; {copyrightYears} VictoriaMetrics.
      {version && <span className="vm-footer__version">&nbsp;Version: {version}</span>}
    </div>
  </footer>;
});

export default Footer;
