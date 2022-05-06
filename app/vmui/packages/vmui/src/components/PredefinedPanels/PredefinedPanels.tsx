import React, {FC, useEffect, useMemo, useRef, useState} from "preact/compat";
import Box from "@mui/material/Box";
import {PanelSettings} from "../../types";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import Typography from "@mui/material/Typography";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {AxisRange, YaxisState} from "../../state/graph/reducer";
import GraphView from "../CustomPanel/Views/GraphView";
import Alert from "@mui/material/Alert";
import {useFetchQuery} from "../../hooks/useFetchQuery";
import Spinner from "../common/Spinner";
import StepConfigurator from "../CustomPanel/Configurator/Query/StepConfigurator";
import GraphSettings from "../CustomPanel/Configurator/Graph/GraphSettings";
import {CustomStep} from "../../state/graph/reducer";
import {marked} from "marked";
import "./dashboard.css";

export interface PredefinedPanelsProps extends PanelSettings {
  filename: string;
}

const PredefinedPanels: FC<PredefinedPanelsProps> = ({
  title,
  description,
  unit,
  expr,
  showLegend,
  filename,
  alias
}) => {

  const {time: {period}} = useAppState();

  const dispatch = useAppDispatch();

  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);
  const [customStep, setCustomStep] = useState<CustomStep>({enable: false, value: period.step || 1});
  const [yaxis, setYaxis] = useState<YaxisState>({
    limits: {
      enable: false,
      range: {"1": [0, 0]}
    }
  });

  const validExpr = useMemo(() => Array.isArray(expr) && expr.every(q => q), [expr]);

  const {isLoading, graphData, error} = useFetchQuery({
    predefinedQuery: validExpr ? expr : [],
    display: "chart",
    visible,
    customStep,
  });

  const setYaxisLimits = (limits: AxisRange) => {
    const tempYaxis = {...yaxis};
    tempYaxis.limits.range = limits;
    setYaxis(tempYaxis);
  };

  const toggleEnableLimits = () => {
    const tempYaxis = {...yaxis};
    tempYaxis.limits.enable = !tempYaxis.limits.enable;
    setYaxis(tempYaxis);
  };

  const setPeriod = ({from, to}: {from: Date, to: Date}) => {
    dispatch({type: "SET_PERIOD", payload: {from, to}});
  };

  useEffect(() => {
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => setVisible(entry.isIntersecting));
    }, { threshold: 0.1 });
    if (containerRef.current) observer.observe(containerRef.current);
    return () => {
      if (containerRef.current) observer.unobserve(containerRef.current);
    };
  }, []);

  if (!validExpr) return <Alert color="error" severity="error" sx={{m: 4}}>
    <code>&quot;expr&quot;</code> not found. Check the configuration file <b>{filename}</b>.
  </Alert>;

  return <Box border="1px solid" borderRadius="2px" borderColor="divider" width={"100%"} height={"100%"} ref={containerRef}>
    <Box px={2} py={1} display="flex" flexWrap={"wrap"}
      width={"100%"}
      alignItems="center" justifyContent="space-between" borderBottom={"1px solid"} borderColor={"divider"}>
      <Tooltip arrow componentsProps={{tooltip: {sx: {maxWidth: "100%"}}}}
        title={<Box sx={{p: 1}}>
          {description && <Box mb={2}>
            <Typography fontWeight={"500"} sx={{mb: 0.5, textDecoration: "underline"}}>Description:</Typography>
            <div className="panelDescription" dangerouslySetInnerHTML={{__html: marked.parse(description)}}/>
          </Box>}
          <Box>
            <Typography fontWeight={"500"} sx={{mb: 0.5, textDecoration: "underline"}}>Queries:</Typography>
            <div>
              {expr.map((e, i) => <Box key={`${i}_${e}`} mb={0.5}>{e}</Box>)}
            </div>
          </Box>
        </Box>}>
        <InfoIcon color="info" sx={{mr: 1}}/>
      </Tooltip>
      <Typography component={"div"} variant="subtitle1" fontWeight={500} sx={{mr: 2, py: 1, flexGrow: "1"}}>
        {title || ""}
      </Typography>
      <Box mr={2} py={1}>
        <StepConfigurator defaultStep={period.step} customStepEnable={customStep.enable}
          setStep={(value) => setCustomStep({...customStep, value: value})}
          toggleEnableStep={() => setCustomStep({...customStep, enable: !customStep.enable})}/>
      </Box>
      <GraphSettings yaxis={yaxis} setYaxisLimits={setYaxisLimits} toggleEnableLimits={toggleEnableLimits}/>
    </Box>
    <Box px={2} pb={2}>
      {isLoading && <Spinner isLoading={true} height={"500px"}/>}
      {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>{error}</Alert>}
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
        setPeriod={setPeriod}/>
      }
    </Box>
  </Box>;
};

export default PredefinedPanels;
