import uPlot, { Series } from "uplot";

export interface HideSeriesArgs {
    hideSeries: string[],
    legend: LegendItemType,
    metaKey: boolean,
    series: Series[]
}

export interface DragArgs {
    e: MouseEvent,
    u: uPlot,
    factor: number,
    setPanning: (enable: boolean) => void,
    setPlotScale: ({ u, min, max }: { u: uPlot, min: number, max: number }) => void
}

export interface LegendItemType {
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
