import { FC, useMemo } from "preact/compat";
import { InstantMetricResult, Logs } from "../../../api/types";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { TopQuery } from "../../../types";
import Button from "../../Main/Button/Button";
import "./style.scss";

export interface JsonViewProps {
  data: InstantMetricResult[] | TopQuery[] | Logs[];
}

const JsonView: FC<JsonViewProps> = ({ data }) => {
  const copyToClipboard = useCopyToClipboard();

  const formattedJson = useMemo(() => JSON.stringify(data, null, 2), [data]);

  const handlerCopy = async () => {
    await copyToClipboard(formattedJson, "Formatted JSON has been copied");
  };

  return (
    <div className="vm-json-view">
      <div className="vm-json-view__copy">
        <Button
          variant="outlined"
          onClick={handlerCopy}
        >
          Copy JSON
        </Button>
      </div>
      <pre className="vm-json-view__code">
        <code>
          {formattedJson}
        </code>
      </pre>
    </div>
  );
};

export default JsonView;
