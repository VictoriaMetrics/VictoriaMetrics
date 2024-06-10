import { getFromStorage, saveToStorage, StorageKeys } from "../../../utils/storage";
import { QueryHistoryType } from "../../../state/query/reducer";
import { MAX_QUERIES_HISTORY, MAX_QUERY_FIELDS } from "../../../constants/graph";

export const getQueriesFromStorage = (key: StorageKeys) => {
  const list = getFromStorage(key) as string;
  return list ? JSON.parse(list) as string[][] : [];
};

export const setQueriesToStorage = (history: QueryHistoryType[]) => {
  // For localStorage, avoid splitting into query fields because when working from multiple tabs can cause confusion.
  // For convenience, we maintain the original structure of `string[][]`
  const lastValues = history.map(h => h.values[h.index]);
  const storageValues = getQueriesFromStorage("QUERY_HISTORY");
  if (!storageValues[0]) storageValues[0] = [];

  const values = storageValues[0];
  const TOTAL_LIMIT = MAX_QUERIES_HISTORY * MAX_QUERY_FIELDS;

  lastValues.forEach((v) => {
    const already = values.includes(v);
    if (!already && v) values.unshift(v);
    if (values.length > TOTAL_LIMIT) values.shift();
  });

  saveToStorage("QUERY_HISTORY", JSON.stringify(storageValues));
};
