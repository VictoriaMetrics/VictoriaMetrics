import Button from "../../Main/Button/Button";
import { ArrowDownIcon } from "../../Main/Icons";
import "./style.scss";
import classNames from "classnames";

interface PaginationProps {
  page: number;
  totalPages: number;
  totalRules: number;
  totalGroups: number;
  pageRules: number;
  pageGroups: number;
  onPageChange: (num: number) => () => void;
}

const getButtons = (page: number, totalPages: number) => {
  const result: number[] = [];
  if (totalPages < 2) return result;
  result.push(1);
  if (page > 3) result.push(0);
  if (page > 2) result.push(page - 1);
  if (page > 1 && page < totalPages) result.push(page);
  if (page > 0 && page < totalPages - 1) result.push(page + 1);
  if (totalPages - page > 2) result.push(0);
  result.push(totalPages);
  return result;
}

const Pagination = ({
  page,
  totalPages,
  onPageChange,
  totalGroups,
  totalRules,
  pageGroups,
  pageRules,
}: PaginationProps) => {

  const buttons = getButtons(page, totalPages);
  return (
    <>
      {!!buttons.length && (
        <div
          className="vm-pagination"
        >
          <span className="vm-pagination-stats">
            <span>Page rules/groups:</span> <b>{pageRules}</b> / <b>{pageGroups}</b>
          </span>
          <div className="vm-pagination-buttons">
            <Button
              className="vm-button-borderless vm-pagination-prev"
              size="small"
              color="gray"
              disabled={page == 1}
              variant="outlined"
              startIcon={<ArrowDownIcon />}
              onClick={onPageChange(page-1)}
            />
            {buttons.map((button, index) => {
              return button ? (
                <Button
                  className={classNames({
                    "vm-button-borderless": page !== button,
                  })}
                  key={index}
                  size="small"
                  color="gray"
                  variant="outlined"
                  onClick={onPageChange(button)}
                >{button}</Button>
              ) : (
                <span className="vm-pagination-more">...</span>
              )
            })}
            <Button
              className="vm-button-borderless vm-pagination-next"
              size="small"
              color="gray"
              disabled={page==totalPages}
              variant="outlined"
              startIcon={<ArrowDownIcon />}
              onClick={onPageChange(page+1)}
            />
          </div>
          <span className="vm-pagination-stats">
            <span>Total rules/groups:</span> <b>{totalRules}</b> / <b>{totalGroups}</b>
          </span>
        </div>
      )}
    </>
  );
};

export default Pagination;
