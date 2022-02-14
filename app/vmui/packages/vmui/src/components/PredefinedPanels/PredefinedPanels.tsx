import React, {FC, useEffect, useRef, useState} from "preact/compat";
import Box from "@mui/material/Box";
import {PanelSettings} from "../../types";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import Typography from "@mui/material/Typography";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {useGraphDispatch, useGraphState} from "../../state/graph/GraphStateContext";
import {AxisRange} from "../../state/graph/reducer";
import GraphView from "../Home/Views/GraphView";
import Alert from "@mui/material/Alert";
import {useFetchQuery} from "../../hooks/useFetchQuery";
import Spinner from "../common/Spinner";
import StepConfigurator from "../Home/Configurator/Query/StepConfigurator";
import GraphSettings from "../Home/Configurator/Graph/GraphSettings";

const PredefinedPanels: FC<PanelSettings> = ({
  title,
  description,
  unit,
  expr,
  hideLegend
}) => {

  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);

  const {isLoading, graphData, error} = useFetchQuery({predefinedQuery: expr, visible, display: "chart"});

  const {time: {period}} = useAppState();
  const {customStep, yaxis} = useGraphState();

  const dispatch = useAppDispatch();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: limits});
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

  return <Box border="1px solid" borderRadius="2px" borderColor="divider" ref={containerRef}>
    <Box px={2} py={1} display="grid" gap={1} gridTemplateColumns="18px 1fr auto"
      alignItems="center" justifyContent="space-between" borderBottom={"1px solid"} borderColor={"divider"}>
      <Tooltip arrow
        title={<Box sx={{p: 1}}>
          {description && <Box mb={2}>
            <Typography fontWeight={"500"} sx={{mb: 0.5, textDecoration: "underline"}}>Description:</Typography>
            {description}
          </Box>}
          <Box>
            <Typography fontWeight={"500"} sx={{mb: 0.5, textDecoration: "underline"}}>Queries:</Typography>
            {expr.map((e, i) => <Box key={`${i}_${e}`} mb={0.5}>{e}</Box>)}
          </Box>
        </Box>}>
        <InfoIcon color="info"/>
      </Tooltip>
      <Typography variant="subtitle1" gridColumn={2} textAlign={"left"} width={"100%"} fontWeight={500}>
        {title || ""}
      </Typography>
      <Box display={"grid"} gridTemplateColumns={"repeat(2, auto)"} gap={2} alignItems={"center"}>
        <StepConfigurator/>
        <GraphSettings/>
      </Box>
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
        hideLegend={hideLegend}
        setYaxisLimits={setYaxisLimits}
        setPeriod={setPeriod}/>
      }
    </Box>
  </Box>;
};

export default PredefinedPanels;