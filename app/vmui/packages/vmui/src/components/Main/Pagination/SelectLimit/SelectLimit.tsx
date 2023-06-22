import React, { FC, useRef } from "preact/compat";
import Tooltip from "../../Tooltip/Tooltip";
import Button from "../../Button/Button";
import { ArrowDropDownIcon } from "../../Icons";
import useBoolean from "../../../../hooks/useBoolean";
import Popper from "../../Popper/Popper";
import classNames from "classnames";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import "./style.scss";

interface SelectLimitProps {
  tooltip?: string;
  limit: number | string;
  onChange: (val: number) => void;
}

const defaultLimits = [10, 25, 50, 100, 250, 500, 1000];

const SelectLimit: FC<SelectLimitProps> = ({ limit, tooltip, onChange }) => {
  const { isMobile } = useDeviceDetect();
  const title = tooltip || "Rows per page";
  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openList,
    toggle: toggleOpenList,
    setFalse: handleClose,
  } = useBoolean(false);

  const handleChangeLimit = (n: number) => () => {
    onChange(n);
    handleClose();
  };

  return (
    <>
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            endIcon={<ArrowDropDownIcon/>}
            onClick={toggleOpenList}
          >
            {limit}
          </Button>
        </div>
      </Tooltip>
      <Popper
        open={openList}
        onClose={handleClose}
        placement="bottom-right"
        buttonRef={buttonRef}
      >
        <div
          className="vm-select-limits"
        >
          {defaultLimits.map(n => (
            <div
              className={classNames({
                "vm-list-item": true,
                "vm-list-item_mobile": isMobile,
                "vm-list-item_active": n === limit,
              })}
              key={n}
              onClick={handleChangeLimit(n)}
            >
              {n}
            </div>
          ))}
        </div>
      </Popper>
    </>
  );
};

export default SelectLimit;
