import { FC, RefObject, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { InstantMetricResult } from "../../../api/types";
import TableView from "../../../components/Views/TableView/TableView";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import { getColumns } from "../../../hooks/useSortedCategories";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";

type Props = {
  liveData: InstantMetricResult[];
  controlsRef: RefObject<HTMLDivElement | null>;
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
