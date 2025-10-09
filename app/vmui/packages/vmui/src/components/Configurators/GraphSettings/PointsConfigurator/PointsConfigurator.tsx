import { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";

interface Props {
  showAllPoints: boolean;
  onChangeShow: (value: boolean) => void;
}

const PointsConfigurator: FC<Props> = ({ showAllPoints, onChangeShow }) => {
  const { isMobile } = useDeviceDetect();

  return (
    <>
      <div className="vm-graph-settings-row">
        <span className="vm-graph-settings-row__label">Show all data points</span>
        <Switch
          value={showAllPoints}
          onChange={onChangeShow}
          label={showAllPoints ? "Enabled" : "Disabled"}
          fullWidth={isMobile}
        />
        <span className="vm-legend-configs-item__info">
          Display every data point, even when no line can be drawn.
        </span>
      </div>
    </>
  );
};

export default PointsConfigurator;
