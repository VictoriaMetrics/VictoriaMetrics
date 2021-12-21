import uPlot, {Series} from "uplot";
import {MetricResult} from "../../api/types";

export interface SetupTooltip {
    u: uPlot,
    metrics: MetricResult[],
    series: Series[],
    tooltip: HTMLDivElement,
    tooltipOffset: {
        left: number,
        top: number
    },
    tooltipIdx: {
        seriesIdx: number | null,
        dataIdx: number | undefined
    }
}

export interface HideSeriesArgs {
    hideSeries: string[],
    legend: LegendItem,
    metaKey: boolean,
    series: Series[]
}

export interface DragArgs {
    e: MouseEvent,
    u: uPlot,
    factor: number,
    setPanning: (enable: boolean) => void,
    setPlotScale: ({u, min, max}: { u: uPlot, min: number, max: number }) => void
}

export interface LegendItem {
    group: number;
    label: string;
    color: string;
    checked: boolean;
}