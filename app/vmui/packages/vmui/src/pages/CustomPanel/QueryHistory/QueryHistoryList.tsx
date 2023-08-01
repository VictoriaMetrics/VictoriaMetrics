import React, { FC, useMemo } from "preact/compat";
import Button from "../../../components/Main/Button/Button";
import { ClockIcon, CopyIcon, PlayCircleOutlineIcon } from "../../../components/Main/Icons";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { QueryHistory } from "../../../state/query/reducer";
import useBoolean from "../../../hooks/useBoolean";
import Modal from "../../../components/Main/Modal/Modal";
import "./style.scss";
import Tabs from "../../../components/Main/Tabs/Tabs";
import { useState } from "react";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";

interface QueryHistoryProps {
  history: QueryHistory[];
  handleSelectQuery: (query: string, index: number) => void
}

const QueryHistoryList: FC<QueryHistoryProps> = ({ history, handleSelectQuery }) => {
  const { isMobile } = useDeviceDetect();
  const copyToClipboard = useCopyToClipboard();
  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  const [activeTab, setActiveTab] = useState("0");
  const tabs = useMemo(() => history.map((item, i) => ({
    value: `${i}`,
    label: `Query ${i+1}`,
  })), [history]);

  const queries = useMemo(() => {
    const historyItem = history[+activeTab];
    return historyItem ? historyItem.values.filter(q => q).reverse() : [];
  }, [activeTab, history]);

  const handleCopyQuery = (value: string) => async () => {
    await copyToClipboard(value, "Query has been copied");
  };

  const handleRunQuery = (value: string, index: number) => () => {
    handleSelectQuery(value, index);
    handleCloseModal();
  };

  return (
    <>
      <Tooltip title={"Show history"}>
        <Button
          color="primary"
          variant="text"
          onClick={handleOpenModal}
          startIcon={<ClockIcon/>}
        />
      </Tooltip>

      {openModal && (
        <Modal
          title={"Query history"}
          onClose={handleCloseModal}
        >
          <div className="vm-query-history">
            <div
              className={classNames({
                "vm-query-history__tabs": true,
                "vm-section-header__tabs": true,
                "vm-query-history__tabs_mobile": isMobile,
              })}
            >
              <Tabs
                activeItem={activeTab}
                items={tabs}
                onChange={setActiveTab}
              />
            </div>
            <div className="vm-query-history-list">
              {queries.map((query, index) => (
                <div
                  className="vm-query-history-list-item"
                  key={index}
                >
                  <span className="vm-query-history-list-item__value">{query}</span>
                  <div className="vm-query-history-list-item__buttons">
                    <Tooltip title={"Execute query"}>
                      <Button
                        size="small"
                        variant="text"
                        onClick={handleRunQuery(query, +activeTab)}
                        startIcon={<PlayCircleOutlineIcon/>}
                      />
                    </Tooltip>
                    <Tooltip title={"Copy query"}>
                      <Button
                        size="small"
                        variant="text"
                        onClick={handleCopyQuery(query)}
                        startIcon={<CopyIcon/>}
                      />
                    </Tooltip>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default QueryHistoryList;
