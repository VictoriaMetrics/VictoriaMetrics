import { CodeIcon, IssueIcon, WikiIcon } from "../components/Main/Icons";

const issueLink = {
  href: "https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new/choose",
  Icon: IssueIcon,
  title: "Create an issue",
};

export const footerLinksByDefault = [
  {
    href: "https://docs.victoriametrics.com/MetricsQL.html",
    Icon: CodeIcon,
    title: "MetricsQL",
  },
  {
    href: "https://docs.victoriametrics.com/#vmui",
    Icon: WikiIcon,
    title: "Documentation",
  },
  issueLink
];

export const footerLinksToLogs = [
  {
    href: "https://docs.victoriametrics.com/victorialogs/logsql/",
    Icon: CodeIcon,
    title: "LogsQL",
  },
  {
    href: "https://docs.victoriametrics.com/victorialogs/",
    Icon: WikiIcon,
    title: "Documentation",
  },
  issueLink
];
