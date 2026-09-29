import { FC, useMemo } from "preact/compat";
import { markdownToSafeHtml, sanitizeHtml } from "../../../utils/html";

type SafeHtmlTagName = keyof HTMLElementTagNameMap;
type SafeHtmlFormat = "html" | "markdown";

interface SafeHtmlProps {
  value: string;
  tagName?: SafeHtmlTagName;
  format?: SafeHtmlFormat;
  className?: string;
}

const SafeHtml: FC<SafeHtmlProps> = ({
  value,
  tagName: Tag = "div",
  format = "html",
  className,
}) => {
  const html = useMemo(() => {
    return format === "markdown" ? markdownToSafeHtml(value) : sanitizeHtml(value);
  }, [format, value]);

  return (
    <Tag
      className={className}
      /* eslint-disable-next-line no-restricted-syntax */
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
};

export default SafeHtml;
