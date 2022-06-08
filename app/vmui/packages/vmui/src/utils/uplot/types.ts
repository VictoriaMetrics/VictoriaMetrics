import uPlot, {Series} from "uplot";
import {MetricResult} from "../../api/types";

export interface SetupTooltip {
    u: uPlot,
    metrics: MetricResult[],
    series: Series[],
    tooltip: HTMLDivElement,
    unit?: string,
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
    freeFormFields: {[key: string]: string};
}

export interface BarSeriesItem {
    which: number[],
    ori: number,
    dir: number,
    radius: number,
    disp: Disp
}

export interface Disp {
    stroke: Stroke,
    fill: Fill,
}

export interface Stroke {
    unit: number,
    values: (u: { data: number[][]; }) => string[],
}

export interface Fill {
    unit: number,
    values: (u: { data: number[][]; }) => string[],
}
