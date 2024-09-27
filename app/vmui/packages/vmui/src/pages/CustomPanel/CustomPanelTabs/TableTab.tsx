import React, { FC } from "react";
import { InstantMetricResult } from "../../../api/types";
import { createPortal, useMemo, useState } from "preact/compat";
import TableView from "../../../components/Views/TableView/TableView";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import { getColumns } from "../../../hooks/useSortedCategories";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";

type Props = {
  liveData: InstantMetricResult[];
  controlsRef: React.RefObject<HTMLDivElement>;
}

const TableTab: FC<Props> = ({ liveData, controlsRef }) => {
  const { tableCompact } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const [displayColumns, setDisplayColumns] = useState<string[]>();

  const columns = useMemo(() => getColumns(liveData || []).map(c => c.key), [liveData]);

  const toggleTableCompact = () => {
    customPanelDispatch({ type: "TOGGLE_TABLE_COMPACT" });
  };

  const controls = (
    <TableSettings
      columns={columns}
      selectedColumns={displayColumns}
      onChangeColumns={setDisplayColumns}
      tableCompact={tableCompact}
      toggleTableCompact={toggleTableCompact}
    />
  );

  return (
    <>
      {controlsRef.current && createPortal(controls, controlsRef.current)}
      <TableView
        data={liveData}
        displayColumns={displayColumns}
      />
    </>
  );
};

export default TableTab;
