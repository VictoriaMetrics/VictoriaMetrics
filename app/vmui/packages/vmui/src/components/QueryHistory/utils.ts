import {
  getFromStorage,
  saveToStorage,
  StorageKeys,
} from "../../utils/storage";
import { QueryHistoryType } from "../../state/query/reducer";
import { MAX_QUERIES_HISTORY, MAX_QUERY_FIELDS } from "../../constants/graph";

export type HistoryKey = Extract<StorageKeys, "METRICS_QUERY_HISTORY">;
export type HistoryType = "QUERY_HISTORY" | "QUERY_FAVORITES";

const getHistoryFromStorage = (key: HistoryKey) => {
  const list = getFromStorage(key) as string;
  const history: Record<HistoryType, string[][]> = list ? JSON.parse(list) : {};
  return history;
};

const saveHistoryToStorage = (key: HistoryKey, historyType: HistoryType, history: string[][]) => {
  const storageHistory = getHistoryFromStorage(key);
  saveToStorage(key, JSON.stringify({
    ...storageHistory,
    [historyType]: history
  }));
};

export const getQueriesFromStorage = (key: HistoryKey, historyType: HistoryType) => {
  return getHistoryFromStorage(key)[historyType] || [];
};

export const setQueriesToStorage = (key: HistoryKey, history: QueryHistoryType[]) => {
  // For localStorage, avoid splitting into query fields because when working from multiple tabs can cause confusion.
  // For convenience, we maintain the original structure of `string[][]`
  const lastValues = history.map(h => h.values[h.index]);
  const storageHistory = getHistoryFromStorage(key);
  const storageValues = storageHistory["QUERY_HISTORY"] || [];
  if (!storageValues[0]) storageValues[0] = [];

  const values = storageValues[0];
  const TOTAL_LIMIT = MAX_QUERIES_HISTORY * MAX_QUERY_FIELDS;

  lastValues.forEach((v) => {
    const already = values.includes(v);
    if (!already && v) values.unshift(v);
    if (values.length > TOTAL_LIMIT) values.pop();
  });

  const newStorageHistory = {
    ...storageHistory,
    QUERY_HISTORY: [values]
  };

  saveToStorage(key, JSON.stringify(newStorageHistory));
};

export const setFavoriteQueriesToStorage = (key: HistoryKey, favoriteQueries: string[][]) => {
  saveHistoryToStorage(key, "QUERY_FAVORITES", favoriteQueries);
};

export const clearQueryHistoryStorage = (key: HistoryKey, historyType: HistoryType) => {
  const history = getHistoryFromStorage(key);
  saveToStorage(key, JSON.stringify({
    ...history,
    [historyType]: [],
  }));
};

export const getUpdatedHistory = (query: string, queryHistory?: QueryHistoryType): QueryHistoryType => {
  const h = queryHistory || { values: [] };
  const queryEqual = query === h.values[h.values.length - 1];
  const newValues = !queryEqual && query ? [...h.values, query] : h.values;

  // limit the history
  if (newValues.length > MAX_QUERIES_HISTORY) newValues.shift();

  return {
    index: h.values.length - Number(queryEqual),
    values: newValues
  };
};
