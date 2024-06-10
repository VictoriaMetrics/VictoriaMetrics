import { LegendItemType } from "../../../../../types";

export const getFreeFields = (legend: LegendItemType) => {
  const keys = Object.keys(legend.freeFormFields).filter(f => f !== "__name__");

  return keys.map(f => {
    const freeField = `${f}=${JSON.stringify(legend.freeFormFields[f])}`;
    const id = `${legend.label}.${freeField}`;

    return {
      id,
      freeField,
      key: f
    };
  });
};
