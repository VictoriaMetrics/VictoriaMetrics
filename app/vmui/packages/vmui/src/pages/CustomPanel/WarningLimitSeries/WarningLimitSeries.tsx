import classNames from "classnames";
import Button from "../../../components/Main/Button/Button";
import React, { FC, useEffect } from "preact/compat";
import useBoolean from "../../../hooks/useBoolean";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Alert from "../../../components/Main/Alert/Alert";

type Props = {
  warning: string;
  query: string[];
  onChange: (show: boolean) => void
}

const WarningLimitSeries: FC<Props> = ({ warning, query, onChange }) => {
  const { isMobile } = useDeviceDetect();

  const {
    value: showAllSeries,
    setTrue: handleShowAll,
    setFalse: resetShowAll,
  } = useBoolean(false);

  useEffect(resetShowAll, [query]);

  useEffect(() => {
    onChange(showAllSeries);
  }, [showAllSeries]);

  return (
    <Alert variant="warning">
      <div
        className={classNames({
          "vm-custom-panel__warning": true,
          "vm-custom-panel__warning_mobile": isMobile
        })}
      >
        <p>{warning}</p>
        <Button
          color="warning"
          variant="outlined"
          onClick={handleShowAll}
        >
        Show all
        </Button>
      </div>
    </Alert>
  );
};

export default WarningLimitSeries;
