import { FC, useEffect, useState } from "preact/compat";
import { getFromStorage, saveToStorage } from "../../../../utils/storage";
import Switch from "../../../Main/Switch/Switch";
import { LEGEND_COLLAPSE_SERIES_LIMIT } from "../../../../constants/graph";
import "./style.scss";

const LegendCollapseController: FC = () => {
  const storageCollapse = getFromStorage("LEGEND_AUTO_COLLAPSE");
  const [legendCollapse, setLegendCollapse] = useState(storageCollapse ? storageCollapse === "true" : true);

  useEffect(() => {
    saveToStorage("LEGEND_AUTO_COLLAPSE", `${legendCollapse}`);
  }, [legendCollapse]);

  return (
    <div className="vm-legend-collapse-controller">
      <Switch
        fullWidth
        color="neutral"
        value={legendCollapse}
        onChange={setLegendCollapse}
        label={<span className="vm-server-configurator__title">Auto-collapse legend</span>}
      />
      <span className="vm-legend-collapse-controller__description">
        Collapses the legend when series count exceeds {LEGEND_COLLAPSE_SERIES_LIMIT} to reduce UI load.
      </span>
    </div>);
};

export default LegendCollapseController;
