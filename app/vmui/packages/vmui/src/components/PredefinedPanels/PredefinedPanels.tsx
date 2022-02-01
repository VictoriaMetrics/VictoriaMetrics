import React, {FC, useEffect, useState} from "preact/compat";
import Box from "@mui/material/Box";
import {PanelSettings} from "../../types";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import Typography from "@mui/material/Typography";
import {MetricBase, MetricResult} from "../../api/types";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {useGraphDispatch, useGraphState} from "../../state/graph/GraphStateContext";
import {AxisRange} from "../../state/graph/reducer";
import {getAppModeEnable, getAppModeParams} from "../../utils/app-mode";
import {getQueryRangeUrl} from "../../api/query-range";
import GraphView from "../Home/Views/GraphView";

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

const PredefinedPanels: FC<PanelSettings> = ({title, description, unit, expr}) => {

  const {serverUrl, time: {period}, queryControls: {nocache}} = useAppState();
  const { customStep, yaxis } = useGraphState();

  const dispatch = useAppDispatch();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: limits});
  };

  const setPeriod = ({from, to}: {from: Date, to: Date}) => {
    dispatch({type: "SET_PERIOD", payload: {from, to}});
  };

  const [graphData, setGraphData] = useState<MetricResult[]>();

  const fetchData = async () => {
    const server = appModeEnable ? appServerUrl : serverUrl;
    const urls = expr.map(q => getQueryRangeUrl(server, q, period, nocache));

    const tempData = [];
    let counter = 1;
    const responses = await Promise.all(urls.map(url => fetch(url)));
    for await (const response of responses) {
      const resp = await response.json();
      tempData.push(...resp.data.result.map((d: MetricBase) => {
        d.group = counter;
        return d;
      }));
      counter++;
    }
    setGraphData(tempData);
  };

  useEffect(() => {
    // TODO add throttled fetch and check effect with yaxis
    fetchData();
  }, [period, expr]); // [period, expr, yaxis]

  return <Box p={1} border="1px solid" borderRadius="2px" borderColor="divider">
    <Box display="grid" gridTemplateColumns="18px 1fr" alignItems="center" justifyContent="space-between">
      {description && <Tooltip title={description} arrow><InfoIcon color="info"/></Tooltip>}
      {title && <Typography variant="subtitle1" gridColumn={2} textAlign={"center"} width={"100%"}>
        {title} ({unit})
      </Typography>}
    </Box>
    <Box>
      {/* TODO add alert with error */}
      {graphData && <GraphView
        data={graphData}
        period={period}
        customStep={customStep}
        query={expr}
        yaxis={yaxis}
        setYaxisLimits={setYaxisLimits}
        setPeriod={setPeriod}/>
      }
    </Box>
  </Box>;
};

export default PredefinedPanels;