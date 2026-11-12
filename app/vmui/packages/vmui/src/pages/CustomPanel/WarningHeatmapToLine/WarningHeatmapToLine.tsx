import { FC } from "preact/compat";
import Alert from "../../../components/Main/Alert/Alert";
import { useGraphState } from "../../../state/graph/GraphStateContext";
import {
  useChangeDisplayMode
} from "../../../components/Configurators/GraphSettings/GraphTypeSwitcher/useChangeDisplayMode";
import Button from "../../../components/Main/Button/Button";
import "./style.scss";

const WarningHeatmapToLine:FC = () => {
  const { isEmptyHistogram } = useGraphState();
  const { handleChange } = useChangeDisplayMode();

  if (!isEmptyHistogram) return null;

  return (
    <Alert
      variant="warning"
      title="Unable to display heatmap"
    >
      <div className="vm-warning-heatmap-to-line">
        <p className="vm-warning-heatmap-to-line__text">
          Switch to a line chart, disable the heatmap in the &quot;Graph settings&quot;,
          or modify the expression to return histogram data.
        </p>

        <Button
          size="small"
          color="primary"
          variant="text"
          onClick={() => handleChange(false)}
        >
          Switch to line chart
        </Button>
      </div>
    </Alert>
  );
};

export default WarningHeatmapToLine;
