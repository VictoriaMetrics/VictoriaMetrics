import React, {FC, useEffect, useMemo, useState} from "react";
import {MetricResult} from "../../../api/types";

import {schemeCategory10, scaleOrdinal, interpolateRainbow, range as d3Range} from "d3";

import {LineChart} from "../../LineChart/LineChart";
import {DataSeries, TimeParams} from "../../../types";
import {getNameForMetric} from "../../../utils/metric";
import {Legend, LegendItem} from "../../Legend/Legend";
import {useSortedCategories} from "../../../hooks/useSortedCategories";
import {InlineBtn} from "../../common/InlineBtn";

export interface GraphViewProps {
  data: MetricResult[];
  timePresets: TimeParams
}

const preDefinedScale = schemeCategory10;

const initialMaxAmount = 20;
const showingIncrement = 20;

const GraphView: FC<GraphViewProps> = ({data, timePresets}) => {

  const [showN, setShowN] = useState(initialMaxAmount);

  const series: DataSeries[] = useMemo(() => {
    return data?.map(d => ({
      metadata: {
        name: getNameForMetric(d)
      },
      metric: d.metric,
      // VM metrics are tuples - much simpler to work with objects in chart
      values: d.values.map(v => ({
        key: v[0],
        value: +v[1]
      }))
    }));
  }, [data]);

  const showingSeries = useMemo(() => series.slice(0 ,showN), [series, showN]);

  const sortedCategories = useSortedCategories(data);

  const seriesNames = useMemo(() => showingSeries.map(s => s.metadata.name), [showingSeries]);

  // should not change as often as array of series names (for instance between executions of same query) to
  // keep related state (like selection of a labels)
  const [seriesNamesStable, setSeriesNamesStable] = useState(seriesNames);

  useEffect(() => {
    // primitive way to check the fact that array contents are identical
    if (seriesNamesStable.join(",") !== seriesNames.join(",")) {
      setSeriesNamesStable(seriesNames);
    }
  }, [seriesNames, setSeriesNamesStable, seriesNamesStable]);

  const amountOfSeries = useMemo(() => series.length, [series]);

  const color = useMemo(() => {
    const len = seriesNamesStable.length;
    const scheme = len <= preDefinedScale.length
      ? preDefinedScale
      : d3Range(len).map(d => d / len).map(interpolateRainbow); // dynamically generate n colors
    return scaleOrdinal<string>()
      .domain(seriesNamesStable) // associate series names with colors
      .range(scheme);
  }, [seriesNamesStable]);


  // changes only if names of series are different
  const initLabels = useMemo(() => {
    return seriesNamesStable.map(name => ({
      color: color(name),
      seriesName: name,
      labelData: showingSeries.find(s => s.metadata.name === name)?.metric, // find is O(n) - can do faster
      checked: true // init with checked always
    } as LegendItem));
  }, [color, seriesNamesStable]);

  const [labels, setLabels] = useState(initLabels);

  useEffect(() => {
    setLabels(initLabels);
  }, [initLabels]);

  const visibleNames = useMemo(() => labels.filter(l => l.checked).map(l => l.seriesName), [labels]);

  const visibleSeries = useMemo(() => showingSeries.filter(s => visibleNames.includes(s.metadata.name)), [showingSeries, visibleNames]);

  const onLegendChange = (index: number) => {
    setLabels(prevState => {
      if (prevState) {
        const newState = [...prevState];
        newState[index] = {...newState[index], checked: !newState[index].checked};
        return newState;
      }
      return prevState;
    });
  };

  return <>
    {(amountOfSeries > 0)
      ? <>
        {amountOfSeries > initialMaxAmount && <div style={{textAlign: "center"}}>
          {amountOfSeries > showN
            ? <span style={{fontStyle: "italic"}}>Showing only first {showN} of {amountOfSeries} series.&nbsp;
              {showN + showingIncrement >= amountOfSeries
                ?
                <InlineBtn handler={() => setShowN(amountOfSeries)} text="Show all"/>
                :
                <>
                  <InlineBtn handler={() => setShowN(prev => Math.min(prev + showingIncrement, amountOfSeries))} text={`Show ${showingIncrement} more`}/>,&nbsp;
                  <InlineBtn handler={() => setShowN(amountOfSeries)} text="show all"/>.
                </>}
            </span>
            : <span style={{fontStyle: "italic"}}>Showing all series.&nbsp;
              <InlineBtn handler={() => setShowN(initialMaxAmount)} text={`Show only ${initialMaxAmount}`}/>.
            </span>}
        </div>}
        <LineChart height={400} series={visibleSeries} color={color} timePresets={timePresets} categories={sortedCategories}></LineChart>
        <Legend labels={labels} onChange={onLegendChange} categories={sortedCategories}></Legend>
      </>
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;