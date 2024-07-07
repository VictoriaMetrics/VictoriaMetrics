import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { DisplayType, PanelSettings } from "../../../types";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import GraphView from "../../../components/Views/GraphView/GraphView";
import { useFetchQuery } from "../../../hooks/useFetchQuery";
import Spinner from "../../../components/Main/Spinner/Spinner";
import GraphSettings from "../../../components/Configurators/GraphSettings/GraphSettings";
import { marked } from "marked";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { InfoIcon } from "../../../components/Main/Icons";
import "./style.scss";
import Alert from "../../../components/Main/Alert/Alert";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { useGraphState } from "../../../state/graph/GraphStateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

export interface PredefinedPanelsProps extends PanelSettings {
  filename: string;
}

const PredefinedPanel: FC<PredefinedPanelsProps> = ({
  title,
  description,
  unit,
  expr,
  showLegend,
  filename,
  alias
}) => {
  const { isMobile } = useDeviceDetect();
  const { period } = useTimeState();
  const { customStep } = useGraphState();
  const dispatch = useTimeDispatch();

  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [spanGaps, setSpanGaps] = useState(false);
  const [yaxis, setYaxis] = useState<YaxisState>({
    limits: {
      enable: false,
      range: { "1": [0, 0] }
    }
  });

  const validExpr = useMemo(() => Array.isArray(expr) && expr.every(q => q), [expr]);

  const { isLoading, graphData, error, warning } = useFetchQuery({
    predefinedQuery: validExpr ? expr : [],
    display: DisplayType.chart,
    visible,
    customStep,
  });

  const setYaxisLimits = (limits: AxisRange) => {
    const tempYaxis = { ...yaxis };
    tempYaxis.limits.range = limits;
    setYaxis(tempYaxis);
  };

  const toggleEnableLimits = () => {
    const tempYaxis = { ...yaxis };
    tempYaxis.limits.enable = !tempYaxis.limits.enable;
    setYaxis(tempYaxis);
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    dispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  useEffect(() => {
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => setVisible(entry.isIntersecting));
    }, { threshold: 0.1 });
    if (containerRef.current) observer.observe(containerRef.current);
    return () => {
      if (containerRef.current) observer.unobserve(containerRef.current);
    };
  }, [containerRef]);

  if (!validExpr) return (
    <Alert variant="error">
      <code>&quot;expr&quot;</code> not found. Check the configuration file <b>{filename}</b>.
    </Alert>
  );

  const TooltipContent = () => (
    <div className="vm-predefined-panel-header__description vm-default-styles">
      {description && (
        <>
          <div>
            <span>Description:</span>
            <div dangerouslySetInnerHTML={{ __html: marked(description) as string }}/>
          </div>
          <hr/>
        </>
      )}
      <div>
        <span>Queries:</span>
        <div>
          {expr.map((e, i) => (
            <div key={`${i}_${e}`} >{e}</div>
          ))}
        </div>
      </div>
    </div>
  );

  return <div
    className="vm-predefined-panel"
    ref={containerRef}
  >
    <div className="vm-predefined-panel-header">
      <Tooltip title={<TooltipContent/>}>
        <div className="vm-predefined-panel-header__info">
          <InfoIcon />
        </div>
      </Tooltip>
      <h3 className="vm-predefined-panel-header__title">
        {title || ""}
      </h3>
      <GraphSettings
        yaxis={yaxis}
        setYaxisLimits={setYaxisLimits}
        toggleEnableLimits={toggleEnableLimits}
        spanGaps={{ value: spanGaps, onChange: setSpanGaps }}
      />
    </div>
    <div className="vm-predefined-panel-body">
      {isLoading && <Spinner/>}
      {error && <Alert variant="error">{error}</Alert>}
      {warning && <Alert variant="warning">{warning}</Alert>}
      {graphData && <GraphView
        data={graphData}
        period={period}
        customStep={customStep}
        query={expr}
        yaxis={yaxis}
        unit={unit}
        alias={alias}
        showLegend={showLegend}
        setYaxisLimits={setYaxisLimits}
        setPeriod={setPeriod}
        fullWidth={false}
        height={isMobile ? window.innerHeight * 0.5 : 500}
        spanGaps={spanGaps}
      />
      }
    </div>
  </div>;
};

export default PredefinedPanel;
