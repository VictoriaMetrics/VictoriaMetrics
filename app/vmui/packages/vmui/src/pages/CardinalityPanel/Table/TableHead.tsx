import { MouseEvent } from "react";
import { Data, EnhancedHeaderTableProps } from "./types";
import classNames from "classnames";
import { ArrowDropDownIcon, InfoOutlinedIcon } from "../../../components/Main/Icons";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";

export function EnhancedTableHead(props: EnhancedHeaderTableProps) {
  const { order, orderBy, onRequestSort, headerCells } = props;
  const createSortHandler = (property: keyof Data) => (event: MouseEvent<HTMLTableCellElement>) => {
    onRequestSort(event, property);
  };

  return (
    <thead className="vm-table-header vm-cardinality-panel-table__header">
      <tr className="vm-table__row vm-table__row_header">
        {headerCells.map((headCell) => (
          <th
            className={classNames({
              "vm-table-cell vm-table-cell_header": true,
              "vm-table-cell_sort": headCell.sortable,
              "vm-table-cell_right": headCell.id === "action",
            })}
            key={headCell.id}
            onClick={headCell.sortable ? createSortHandler(headCell.id as keyof Data) : undefined}
          >
            <div className="vm-table-cell__content">
              {headCell.info && (
                <Tooltip title={headCell.info}>
                  <div className="vm-metrics-content-header__tip-icon">
                    <InfoOutlinedIcon/>
                  </div>
                </Tooltip>
              )}
              {headCell.label}
              {headCell.sortable && (
                <div
                  className={classNames({
                    "vm-table__sort-icon": true,
                    "vm-table__sort-icon_active": orderBy === headCell.id,
                    "vm-table__sort-icon_desc": order === "desc" && orderBy === headCell.id
                  })}
                >
                  <ArrowDropDownIcon/>
                </div>
              )}
            </div>
          </th>
        ))}
      </tr>
    </thead>
  );
}
