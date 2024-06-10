import React, { FC } from "react";
import EnhancedTable from "../Table/Table";
import TableCells from "../Table/TableCells/TableCells";
import { Data, HeadCell } from "../Table/types";
import { MutableRef } from "preact/hooks";
import Tabs from "../../../components/Main/Tabs/Tabs";
import { useMemo, useState } from "preact/compat";
import { ChartIcon, InfoIcon, TableIcon } from "../../../components/Main/Icons";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import SimpleBarChart from "../../../components/Chart/SimpleBarChart/SimpleBarChart";

interface MetricsProperties {
  rows: Data[];
  onActionClick: (name: string) => void;
  tabs: string[];
  chartContainer: MutableRef<HTMLDivElement> | undefined;
  totalSeries: number,
  totalSeriesPrev: number,
  sectionTitle: string;
  tip?: string;
  tableHeaderCells: HeadCell[];
  isPrometheus: boolean;
}

const MetricsContent: FC<MetricsProperties> = ({
  rows,
  tabs: tabsProps = [],
  chartContainer,
  totalSeries,
  totalSeriesPrev,
  onActionClick,
  sectionTitle,
  tip,
  tableHeaderCells,
  isPrometheus,
}) => {
  const { isMobile } = useDeviceDetect();
  const [activeTab, setActiveTab] = useState("table");

  const noDataPrometheus = isPrometheus && !rows.length;

  const tableCells = (row: Data) => (
    <TableCells
      row={row}
      totalSeries={totalSeries}
      totalSeriesPrev={totalSeriesPrev}
      onActionClick={onActionClick}
    />
  );

  const tabs = useMemo(() => tabsProps.map((t, i) => ({
    value: t,
    label: t,
    icon: i === 0 ? <TableIcon /> : <ChartIcon />
  })), [tabsProps]);

  return (
    <div
      className={classNames({
        "vm-metrics-content": true,
        "vm-metrics-content_mobile": isMobile,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div className="vm-metrics-content-header vm-section-header">
        <h5
          className={classNames({
            "vm-metrics-content-header__title": true,
            "vm-section-header__title": true,
            "vm-section-header__title_mobile": isMobile,
          })}
        >
          {!isMobile && tip && (
            <Tooltip
              title={<p
                dangerouslySetInnerHTML={{ __html: tip }}
                className="vm-metrics-content-header__tip"
              />}
            >
              <div className="vm-metrics-content-header__tip-icon"><InfoIcon/></div>
            </Tooltip>
          )}
          {sectionTitle}
        </h5>
        <div className="vm-section-header__tabs">
          <Tabs
            activeItem={activeTab}
            items={tabs}
            onChange={setActiveTab}
          />
        </div>
      </div>
      {noDataPrometheus && (
        <div className="vm-metrics-content-prom-data">
          <div className="vm-metrics-content-prom-data__icon"><InfoIcon/></div>
          <h3 className="vm-metrics-content-prom-data__title">
            Prometheus Data Limitation
          </h3>
          <p className="vm-metrics-content-prom-data__text">
            Due to missing data from your Prometheus source, some tables may appear empty.<br/>
            This does not indicate an issue with your system or our tool.
          </p>
        </div>
      )}
      {!noDataPrometheus && activeTab === "table" && (
        <div
          ref={chartContainer}
          className={classNames({
            "vm-metrics-content__table": true,
            "vm-metrics-content__table_mobile": isMobile
          })}
        >
          <EnhancedTable
            rows={rows}
            headerCells={tableHeaderCells}
            defaultSortColumn={"value"}
            tableCells={tableCells}
          />
        </div>
      )}
      {!noDataPrometheus && activeTab === "graph" && (
        <div className="vm-metrics-content__chart">
          <SimpleBarChart data={rows.map(({ name, value }) => ({ name, value }))}/>
        </div>
      )}
    </div>
  );
};

export default MetricsContent;
