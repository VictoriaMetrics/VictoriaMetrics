import { FC } from "preact/compat";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { WarningIcon } from "../../../Main/Icons";

const waringText = "Browser timezone is not recognized, supported, or could not be determined.";

const WarningTimezone: FC = () => {

  return (
    <Tooltip title={waringText}>
      <WarningIcon/>
    </Tooltip>
  );
};

export default WarningTimezone;
