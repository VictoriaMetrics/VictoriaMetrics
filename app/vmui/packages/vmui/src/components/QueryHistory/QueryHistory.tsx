import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import Button from "../Main/Button/Button";
import { ClockIcon, DeleteIcon } from "../Main/Icons";
import Tooltip from "../Main/Tooltip/Tooltip";
import useBoolean from "../../hooks/useBoolean";
import Modal from "../Main/Modal/Modal";
import Tabs from "../Main/Tabs/Tabs";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import useEventListener from "../../hooks/useEventListener";
import { useQueryState } from "../../state/query/QueryStateContext";
import { clearQueryHistoryStorage, getQueriesFromStorage, setFavoriteQueriesToStorage } from "./utils";
import QueryHistoryItem from "./QueryHistoryItem";
import classNames from "classnames";
import "./style.scss";
import { StorageKeys } from "../../utils/storage";
import { arrayEquals } from "../../utils/array";

interface Props {
  handleSelectQuery: (query: string, index: number) => void
  historyKey: Extract<StorageKeys, "LOGS_QUERY_HISTORY" | "METRICS_QUERY_HISTORY">;
}

export const HistoryTabTypes = {
  session: "session",
  storage: "saved",
  favorite: "favorite",
};

export const historyTabs = [
  { label: "Session history", value: HistoryTabTypes.session },
  { label: "Saved history", value: HistoryTabTypes.storage },
  { label: "Favorite queries", value: HistoryTabTypes.favorite },
];

const QueryHistory: FC<Props> = ({ handleSelectQuery, historyKey }) => {
  const { queryHistory: historyState } = useQueryState();
  const { isMobile } = useDeviceDetect();

  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  const [activeTab, setActiveTab] = useState(historyTabs[0].value);
  const [historyStorage, setHistoryStorage] = useState(getQueriesFromStorage(historyKey, "QUERY_HISTORY"));
  const [historyFavorites, setHistoryFavorites] = useState(getQueriesFromStorage(historyKey, "QUERY_FAVORITES"));

  const historySession = useMemo(() => {
    return historyState.map((h) => h.values.filter(q => q).reverse());
  }, [historyState]);

  const list = useMemo(() => {
    switch (activeTab) {
      case HistoryTabTypes.favorite:
        return historyFavorites;
      case HistoryTabTypes.storage:
        return historyStorage;
      default:
        return historySession;
    }
  }, [activeTab, historyFavorites, historyStorage, historySession]);

  const isNoData = list?.every(s => !s.length);

  const noDataText = useMemo(() => {
    switch (activeTab) {
      case HistoryTabTypes.favorite:
        return "Favorites queries are empty.\nTo see your favorites, mark a query as a favorite.";
      default:
        return "Query history is empty.\nTo see the history, please make a query.";
    }
  }, [activeTab]);

  const handleRunQuery = (group: number) => (value: string) => {
    handleSelectQuery(value, group);
    handleCloseModal();
  };

  const handleToggleFavorite = (value: string, isFavorite: boolean) => {
    setHistoryFavorites((prev) => {
      const values = prev[0] || [];
      if (isFavorite) return [values.filter(v => v !== value)];
      if (!isFavorite && !values.includes(value)) return [[...values, value]];
      return prev;
    });
  };

  const updateStageHistory = () => {
    setHistoryStorage(getQueriesFromStorage(historyKey, "QUERY_HISTORY"));
    setHistoryFavorites(getQueriesFromStorage(historyKey, "QUERY_FAVORITES"));
  };

  const handleClearStorage = () => {
    clearQueryHistoryStorage(historyKey, "QUERY_HISTORY");
  };

  useEffect(() => {
    const nextValue = historyFavorites[0] || [];
    const prevValue = getQueriesFromStorage(historyKey, "QUERY_FAVORITES")[0] || [];
    const isEqual = arrayEquals(nextValue, prevValue);
    if (isEqual) return;
    setFavoriteQueriesToStorage(historyKey, historyFavorites);
  }, [historyFavorites]);

  useEventListener("storage", updateStageHistory);

  return (
    <>
      <Tooltip title={"Show history"}>
        <Button
          color="primary"
          variant="text"
          onClick={handleOpenModal}
          startIcon={<ClockIcon/>}
          ariaLabel={"Show history"}
        />
      </Tooltip>

      {openModal && (
        <Modal
          title={"Query history"}
          onClose={handleCloseModal}
        >
          <div
            className={classNames({
              "vm-query-history": true,
              "vm-query-history_mobile": isMobile,
            })}
          >
            <div
              className={classNames({
                "vm-query-history__tabs": true,
                "vm-section-header__tabs": true,
                "vm-query-history__tabs_mobile": isMobile,
              })}
            >
              <Tabs
                activeItem={activeTab}
                items={historyTabs}
                onChange={setActiveTab}
              />
            </div>
            <div className="vm-query-history-list">
              {isNoData && <div className="vm-query-history-list__no-data">{noDataText}</div>}
              {list.map((queries, group) => (
                <div key={group}>
                  {list.length > 1 && (
                    <div
                      className={classNames({
                        "vm-query-history-list__group-title": true,
                        "vm-query-history-list__group-title_first": group === 0,
                      })}
                    >
                      Query {group + 1}
                    </div>
                  )}
                  {queries.map((query, index) => (
                    <QueryHistoryItem
                      key={index}
                      query={query}
                      favorites={historyFavorites.flat()}
                      onRun={handleRunQuery(group)}
                      onToggleFavorite={handleToggleFavorite}
                    />
                  ))}
                </div>
              ))}
              {(activeTab === HistoryTabTypes.storage) && !isNoData && (
                <div className="vm-query-history-footer">
                  <Button
                    color="error"
                    variant="outlined"
                    size="small"
                    startIcon={<DeleteIcon/>}
                    onClick={handleClearStorage}
                  >
                    clear history
                  </Button>
                </div>
              )}
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default QueryHistory;
