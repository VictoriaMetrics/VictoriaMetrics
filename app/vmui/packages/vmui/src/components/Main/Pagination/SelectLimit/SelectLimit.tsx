import { FC, useMemo, useRef } from "preact/compat";
import { ArrowDropDownIcon } from "../../Icons";
import useBoolean from "../../../../hooks/useBoolean";
import Popper from "../../Popper/Popper";
import classNames from "classnames";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import "./style.scss";

interface SelectLimitProps {
  limit: number | string;
  allowUnlimited?: boolean;
  onChange: (val: number) => void;
  onOpenSelect?: () => void;
}

const defaultLimits = [10, 25, 50, 100, 250, 500, 1000];

const SelectLimit: FC<SelectLimitProps> = ({ limit, allowUnlimited, onChange, onOpenSelect }) => {
  const { isMobile } = useDeviceDetect();
  const buttonRef = useRef<HTMLDivElement>(null);

  const limits = useMemo(() => {
    return allowUnlimited ? [...defaultLimits, 0] : defaultLimits;
  }, [allowUnlimited]);

  const {
    value: openList,
    toggle: toggleOpenList,
    setFalse: handleClose,
  } = useBoolean(false);

  const handleClickSelect = () => {
    toggleOpenList();
    if(!openList) onOpenSelect?.();
  };

  const handleChangeLimit = (n: number) => () => {
    onChange(n);
    handleClose();
  };

  return (
    <>
      <div
        className="vm-select-limits-button"
        onClick={handleClickSelect}
        ref={buttonRef}
      >
        <div>
          Rows per page: <b>{limit || "All"}</b>
        </div>
        <ArrowDropDownIcon/>
      </div>
      <Popper
        open={openList}
        onClose={handleClose}
        placement="bottom-right"
        buttonRef={buttonRef}
      >
        <div
          className="vm-select-limits"
        >
          {limits.map(n => (
            <div
              className={classNames({
                "vm-list-item": true,
                "vm-list-item_mobile": isMobile,
                "vm-list-item_active": n === limit,
              })}
              key={n}
              onClick={handleChangeLimit(n)}
            >
              {n || "All"}
            </div>
          ))}
        </div>
      </Popper>
    </>
  );
};

export default SelectLimit;
