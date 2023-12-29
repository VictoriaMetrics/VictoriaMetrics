import uPlot, { Series as uPlotSeries } from "uplot";
import { ForecastType, SeriesItem } from "../../types";
import { anomalyColors, hexToRGB } from "../color";

export const setBand = (plot: uPlot, series: uPlotSeries[]) => {
  // First, remove any existing bands
  plot.delBand();

  // If there aren't at least two series, we can't create a band
  if (series.length < 2) return;

  // Cast and enrich each series item with its index
  const seriesItems = (series as SeriesItem[]).map((s, index) => ({ ...s, index }));

  const upperSeries = seriesItems.filter(s => s.forecast === ForecastType.yhatUpper);
  const lowerSeries = seriesItems.filter(s => s.forecast === ForecastType.yhatLower);

  // Create bands by matching upper and lower series based on their freeFormFields
  const bands = upperSeries.map((upper) => {
    const correspondingLower = lowerSeries.find(lower => lower.forecastGroup === upper.forecastGroup);
    if (!correspondingLower) return null;
    return {
      series: [upper.index, correspondingLower.index] as [number, number],
      fill: createBandFill(ForecastType.yhatUpper),
    };
  }).filter(band => band !== null) as uPlot.Band[]; // Filter out any nulls from failed matches

  // If there are no bands to add, exit the function
  if (!bands.length) return;

  // Add each band to the plot
  bands.forEach(band => {
    plot.addBand(band);
  });
};

// Helper function to create the fill color for a band
function createBandFill(forecastType: ForecastType): string {
  const rgb = hexToRGB(anomalyColors[forecastType]);
  return `rgba(${rgb}, 0.05)`;
}
