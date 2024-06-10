import React, { FC } from "preact/compat";
import Button from "../../Button/Button";
import { ArrowDownIcon } from "../../Icons";
import "./style.scss";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import classNames from "classnames";

interface PaginationControlProps {
  page: number;
  length: number;
  limit: number;
  onChange: (page: number) => void;
}

const PaginationControl: FC<PaginationControlProps> = ({ page, length, limit, onChange }) => {
  const { isMobile } = useDeviceDetect();

  const handleChangePage = (step: number) => () => {
    onChange(+page + step);
    window.scrollTo(0, 0);
  };

  return (
    <div
      className={classNames({
        "vm-pagination": true,
        "vm-pagination_mobile": isMobile
      })}
    >
      {page > 1 && (
        <Button
          variant={"text"}
          onClick={handleChangePage(-1)}
          startIcon={<div className="vm-pagination__icon vm-pagination__icon_prev"><ArrowDownIcon/></div>}
        >
          Previous
        </Button>
      )}
      {length >= limit && (
        <Button
          variant={"text"}
          onClick={handleChangePage(1)}
          endIcon={<div className="vm-pagination__icon vm-pagination__icon_next"><ArrowDownIcon/></div>}
        >
          Next
        </Button>
      )}
    </div>
  );
};

export default PaginationControl;
