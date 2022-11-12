import { FC } from "react";
import EnhancedTable from "../../../components/Main/Table/Table";
import TableCells from "../TableCells/TableCells";
import BarChart from "../../../components/Chart/BarChart/BarChart";
import { barOptions } from "../../../components/Chart/BarChart/consts";
import React, { SyntheticEvent } from "react";
import { Data, HeadCell } from "../../../components/Main/Table/types";
import { MutableRef } from "preact/hooks";
import Tabs from "../../../components/Main/Tabs/Tabs";
import { useMemo } from "preact/compat";
import { ChartIcon, TableIcon } from "../../../components/Main/Icons";
import "./style.scss";

interface MetricsProperties {
  rows: Data[];
  activeTab: number;
  onChange: (newValue: string, tabId: string) => void;
  onActionClick: (e: SyntheticEvent) => void;
  tabs: string[];
  chartContainer: MutableRef<HTMLDivElement> | undefined;
  totalSeries: number,
  tabId: string;
  sectionTitle: string;
  tableHeaderCells: HeadCell[];
}

const MetricsContent: FC<MetricsProperties> = ({
  rows,
  activeTab,
  onChange,
  tabs: tabsProps,
  chartContainer,
  totalSeries,
  tabId,
  onActionClick,
  sectionTitle,
  tableHeaderCells,
}) => {
  const tableCells = (row: Data) => (
    <TableCells
      row={row}
      totalSeries={totalSeries}
      onActionClick={onActionClick}
    />
  );

  const tabs = useMemo(() => tabsProps.map((t, i) => ({
    value: String(i),
    label: t,
    icon: i === 0 ? <TableIcon /> : <ChartIcon />
  })), [tabsProps]);

  const handleChangeTab = (newValue: string) => {
    onChange(newValue, tabId);
  };

  return (
    <div className="vm-metrics-content vm-block">
      <div className="vm-metrics-content-header vm-table-header">
        <h5 className="vm-table-header__title">{sectionTitle}</h5>
        <div className="vvm-table-header__tabs">
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
        </div>
      </div>
      <div
        ref={chartContainer}
        // style={{ width: "100%", paddingRight: idx !== 0 ? "40px" : 0 }}
      >
        {activeTab === 0 && (
          <EnhancedTable
            rows={rows}
            headerCells={tableHeaderCells}
            defaultSortColumn={"value"}
            tableCells={tableCells}
          />
        )}
        {activeTab === 1 && (
          <BarChart
            data={[
              // eslint-disable-next-line @typescript-eslint/ban-ts-comment
              // @ts-ignore
              rows.map((v) => v.name),
              rows.map((v) => v.value),
              rows.map((_, i) => i % 12 == 0 ? 1 : i % 10 == 0 ? 2 : 0),
            ]}
            container={chartContainer?.current || null}
            configs={barOptions}
          />
        )}
      </div>
    </div>
  );
};

export default MetricsContent;
