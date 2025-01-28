import React, { FC } from "preact/compat";
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
    <Alert variant="warning">
      <div className="vm-warning-heatmap-to-line">
        <p className="vm-warning-heatmap-to-line__text">
         The expression cannot be displayed as a heatmap.
         To make the graph work, disable the heatmap in the &quot;Graph settings&quot; or modify the expression.
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
