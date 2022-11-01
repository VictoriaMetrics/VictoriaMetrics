import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import Box from "@mui/material/Box";
import { PanelSettings } from "../../../types";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import Typography from "@mui/material/Typography";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import GraphView from "../../../components/Views/GraphView";
import Alert from "@mui/material/Alert";
import { useFetchQuery } from "../../../hooks/useFetchQuery";
import Spinner from "../../../components/Main/Spinner";
import StepConfigurator from "../../../components/Configurators/AdditionalSettings/StepConfigurator";
import GraphSettings from "../../../components/Configurators/GraphSettings/GraphSettings";
import { marked } from "marked";
import "../dashboard.css";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";

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

  const { period } = useTimeState();
  const dispatch = useTimeDispatch();

  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);
  const [customStep, setCustomStep] = useState<number>(period.step || 1);
  const [yaxis, setYaxis] = useState<YaxisState>({
    limits: {
      enable: false,
      range: { "1": [0, 0] }
    }
  });

  const validExpr = useMemo(() => Array.isArray(expr) && expr.every(q => q), [expr]);

  const { isLoading, graphData, error, warning } = useFetchQuery({
    predefinedQuery: validExpr ? expr : [],
    display: "chart",
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
  }, []);

  if (!validExpr) return <Alert
    color="error"
    severity="error"
    sx={{ m: 4 }}
  >
    <code>&quot;expr&quot;</code> not found. Check the configuration file <b>{filename}</b>.
  </Alert>;

  return <Box
    border="1px solid"
    borderRadius="2px"
    borderColor="divider"
    width={"100%"}
    height={"100%"}
    ref={containerRef}
  >
    <Box
      px={2}
      py={1}
      display="flex"
      flexWrap={"wrap"}
      width={"100%"}
      alignItems="center"
      justifyContent="space-between"
      borderBottom={"1px solid"}
      borderColor={"divider"}
    >
      <Tooltip
        arrow
        componentsProps={{ tooltip: { sx: { maxWidth: "100%" } } }}
        title={<Box sx={{ p: 1 }}>
          {description && <Box mb={2}>
            <Typography
              fontWeight={"500"}
              sx={{ mb: 0.5, textDecoration: "underline" }}
            >Description:</Typography>
            <div
              className="panelDescription"
              dangerouslySetInnerHTML={{ __html: marked.parse(description) }}
            />
          </Box>}
          <Box>
            <Typography
              fontWeight={"500"}
              sx={{ mb: 0.5, textDecoration: "underline" }}
            >Queries:</Typography>
            <div>
              {expr.map((e, i) => <Box
                key={`${i}_${e}`}
                mb={0.5}
              >{e}</Box>)}
            </div>
          </Box>
        </Box>}
      >
        <InfoIcon
          color="info"
          sx={{ mr: 1 }}
        />
      </Tooltip>
      <Typography
        component={"div"}
        variant="subtitle1"
        fontWeight={500}
        sx={{ mr: 2, py: 1, flexGrow: "1" }}
      >
        {title || ""}
      </Typography>
      <Box
        mr={2}
        py={1}
      >
        <StepConfigurator
          defaultStep={period.step}
          setStep={(value) => setCustomStep(value)}
        />
      </Box>
      <GraphSettings
        yaxis={yaxis}
        setYaxisLimits={setYaxisLimits}
        toggleEnableLimits={toggleEnableLimits}
      />
    </Box>
    <Box
      px={2}
      pb={2}
    >
      {isLoading && <Spinner
        isLoading={true}
        height={"500px"}
      />}
      {error && <Alert
        color="error"
        severity="error"
        sx={{ whiteSpace: "pre-wrap", mt: 2 }}
      >{error}</Alert>}
      {warning && <Alert
        color="warning"
        severity="warning"
        sx={{ whiteSpace: "pre-wrap", my: 2 }}
      >{warning}</Alert>}
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
      />
      }
    </Box>
  </Box>;
};

export default PredefinedPanel;
