import React from "react";
import { ArrowDownIcon } from "../Icons";
import { useMemo } from "preact/compat";
import classNames from "classnames";
import "./style.scss";

interface PaginationProps {
  currentPage: number;
  totalItems: number;
  itemsPerPage: number;
  onPageChange: (page: number) => void;
  maxVisiblePages?: number;
}

const Pagination: React.FC<PaginationProps> = ({
  currentPage,
  totalItems,
  itemsPerPage,
  onPageChange,
  maxVisiblePages = 10
}) => {
  const totalPages = Math.ceil(totalItems / itemsPerPage);
  const handlePageChange = (page: number) => {
    if (page < 1 || page > totalPages) return;
    onPageChange(page);
  };

  const pages = useMemo(() => {
    const pages = [];
    if (totalPages <= maxVisiblePages) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      const startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
      const endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);

      if (startPage > 1) {
        pages.push(1);
        if (startPage > 2) {
          pages.push("...");
        }
      }

      for (let i = startPage; i <= endPage; i++) {
        pages.push(i);
      }

      if (endPage < totalPages) {
        if (endPage < totalPages - 1) {
          pages.push("...");
        }
        pages.push(totalPages);
      }
    }
    return pages;
  }, [totalPages, currentPage, maxVisiblePages]);

  const handleClickNav = (stepPage: number) => () => {
    handlePageChange(currentPage + stepPage);
  };

  const handleClickPage = (page: number | string) => () => {
    if (typeof page === "number") {
      handlePageChange(page);
    }
  };

  if (pages.length <= 1) return null;

  return (
    <div className="vm-pagination">
      <button
        className="vm-pagination__page vm-pagination__arrow vm-pagination__arrow_prev"
        onClick={handleClickNav(-1)}
        disabled={currentPage === 1}
      >
        <ArrowDownIcon/>
      </button>
      {pages.map((page, index) => (
        <button
          key={index}
          onClick={handleClickPage(page)}
          className={classNames({
            "vm-pagination__page": true,
            "vm-pagination__page_active": currentPage === page,
            "vm-pagination__page_disabled": page === "..."
          })}
          disabled={page === "..."}
        >
          {page}
        </button>
      ))}
      <button
        className="vm-pagination__page vm-pagination__arrow vm-pagination__arrow_next"
        onClick={handleClickNav(1)}
        disabled={currentPage === totalPages}
      >
        <ArrowDownIcon/>
      </button>
    </div>
  );
};

export default Pagination;
