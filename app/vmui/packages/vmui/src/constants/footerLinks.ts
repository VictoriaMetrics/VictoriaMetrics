import { CodeIcon, IssueIcon, WikiIcon } from "../components/Main/Icons";

const issueLink = {
  href: "https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose",
  Icon: IssueIcon,
  title: "Create an issue",
};

export const footerLinksByDefault = [
  {
    href: "https://docs.victoriametrics.com/victoriametrics/metricsql/",
    Icon: CodeIcon,
    title: "MetricsQL",
  },
  {
    href: "https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui",
    Icon: WikiIcon,
    title: "Documentation",
  },
  issueLink
];
