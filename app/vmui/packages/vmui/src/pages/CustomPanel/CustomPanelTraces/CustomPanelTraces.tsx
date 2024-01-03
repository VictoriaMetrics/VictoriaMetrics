import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import TracingsView from "../../../components/TraceQuery/TracingsView";
import React, { FC, useEffect, useState } from "preact/compat";
import Trace from "../../../components/TraceQuery/Trace";
import { DisplayType } from "../../../types";

type Props = {
  traces?: Trace[];
  displayType: DisplayType;
}

const CustomPanelTraces: FC<Props> = ({ traces, displayType }) => {
  const { isTracingEnabled } = useCustomPanelState();
  const [tracesState, setTracesState] = useState<Trace[]>([]);

  const handleTraceDelete = (trace: Trace) => {
    const updatedTraces = tracesState.filter((data) => data.idValue !== trace.idValue);
    setTracesState([...updatedTraces]);
  };

  useEffect(() => {
    if (traces) {
      setTracesState([...tracesState, ...traces]);
    }
  }, [traces]);

  useEffect(() => {
    setTracesState([]);
  }, [displayType]);

  return <>
    {isTracingEnabled && (
      <div className="vm-custom-panel__trace">
        <TracingsView
          traces={tracesState}
          onDeleteClick={handleTraceDelete}
        />
      </div>
    )}
  </>;
};

export default CustomPanelTraces;
