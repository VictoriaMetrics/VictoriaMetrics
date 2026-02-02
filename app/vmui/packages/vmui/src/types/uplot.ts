import { Axis, Series } from "uplot";

export interface SeriesItemStatsFormatted {
    min: string,
    max: string,
    median: string,
}

export interface SeriesItem extends Series {
    freeFormFields: {[key: string]: string};
    statsFormatted: SeriesItemStatsFormatted;
    median: number;
    hasAlias?: boolean;
}

export interface HideSeriesArgs {
    hideSeries: string[],
    legend: LegendItemType,
    metaKey: boolean,
    series: Series[],
}

export type MinMax = { min: number, max: number }

export type SetMinMax = ({ min, max }: MinMax) => void

export interface LegendItemType {
    group: number;
    label: string;
    color: string;
    checked: boolean;
    freeFormFields: {[key: string]: string};
    statsFormatted: SeriesItemStatsFormatted;
    median: number
    hasAlias: boolean;
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

export type ArrayRGB = [number, number, number]

export interface AxisExtend extends Axis {
    _size?: number;
}
