import React, { FC, useMemo, useState } from "preact/compat";
import { LegendLogHits } from "../../../../api/types";
import { getStreamPairs } from "../../../../utils/logs";
import { CopyIcon, ModalIcon, OpenNewIcon } from "../../../Main/Icons";
import Modal from "../../../Main/Modal/Modal";
import useBoolean from "../../../../hooks/useBoolean";
import { HITS_GROUP_FIELD } from "../../../../pages/ExploreLogs/hooks/useFetchLogHits";
import Button from "../../../Main/Button/Button";
import useCopyToClipboard from "../../../../hooks/useCopyToClipboard";
import { useLocation, useSearchParams } from "react-router-dom";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import TextField from "../../../Main/TextField/TextField";

interface Props {
  legend: LegendLogHits;
  onApplyFilter: (value: string) => void;
  onClose: () => void;
}

const LegendHitsMenuOther: FC<Props> = ({ legend }) => {
  const copyToClipboard = useCopyToClipboard();
  const [searchParams] = useSearchParams();
  const location = useLocation();

  const [search, setSearch] = useState("");

  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  const hits: LegendLogHits[] = useMemo(() => {
    const includesHits = legend?.includesHits || [];
    if (!search) return includesHits;
    return includesHits.filter(({ label }) => label.toLowerCase().includes(search.toLowerCase()));
  }, [legend.includesHits, search]);

  const hitsRows = useMemo(() => {
    return hits.map(hit => {
      const fields = getStreamPairs(hit.label);
      const total = hit.total.toLocaleString("en-US");
      return { label: hit.label, total, fields };
    });
  }, [hits]);

  const createCopyHandler = (copyValue: string) => async () => {
    await copyToClipboard(copyValue, "Row has been copied");
  };

  const createNewPageUrl = (label: string) => {
    const value = `${HITS_GROUP_FIELD}: ${label}`;
    const newSearchParams = new URLSearchParams(searchParams);
    newSearchParams.set("query", `${value}`);
    return `${location.pathname}#/?${newSearchParams.toString()}`;
  };


  return (
    <>
      <div
        className="vm-legend-hits-menu-row vm-legend-hits-menu-row_interactive"
        onClick={handleOpenModal}
      >
        <div className="vm-legend-hits-menu-row__icon">
          <ModalIcon/>
        </div>
        <div className="vm-legend-hits-menu-row__title">
          View all {HITS_GROUP_FIELD}s
        </div>
      </div>
      {openModal && (
        <Modal
          title={"Other hits"}
          onClose={handleCloseModal}
        >
          <div className="vm-legend-hits-menu-other-list">
            <div className="vm-legend-hits-menu-other-list__search">
              <TextField
                autofocus
                label="Search"
                value={search}
                onChange={setSearch}
                type="search"
              />
            </div>
            <table>
              <thead className="vm-legend-hits-menu-other-list-header">
                <tr className="vm-legend-hits-menu-other-list-row vm-legend-hits-menu-other-list-row_header">
                  <th
                    className="vm-legend-hits-menu-other-list-cell vm-legend-hits-menu-other-list-cell_number vm-legend-hits-menu-other-list-cell_header"
                  >
                  total
                  </th>
                  <th className="vm-legend-hits-menu-other-list-cell vm-legend-hits-menu-other-list-cell_header">
                    {HITS_GROUP_FIELD}
                  </th>
                  <th className="vm-legend-hits-menu-other-list-cell vm-legend-hits-menu-other-list-cell_header"/>
                </tr>
              </thead>
              <tbody>
                {hitsRows.map(({ label, total, fields }) => (
                  <tr
                    key={label}
                    className="vm-legend-hits-menu-other-list-row"
                  >
                    <td className="vm-legend-hits-menu-other-list-cell vm-legend-hits-menu-other-list-cell_number">
                      {total}
                    </td>
                    <td className="vm-legend-hits-menu-other-list-cell vm-legend-hits-menu-other-list-cell_fields">
                      <div className="vm-legend-hits-menu-other-list-fields">
                        {fields.map((field) => (
                          <span
                            key={field}
                            className="vm-legend-hits-menu-other-list-fields__field"
                          >{field}</span>
                        ))}
                      </div>
                    </td>
                    <td className="vm-legend-hits-menu-other-list-cell">
                      <div className="vm-legend-hits-menu-other-list-actions">
                        <a
                          href={createNewPageUrl(label)}
                          target={"_blank"}
                          rel="noreferrer"
                        >
                          <Tooltip title="Opens a new window">
                            <Button
                              size="small"
                              variant="text"
                              startIcon={<OpenNewIcon/>}
                              ariaLabel="open new page"
                            />
                          </Tooltip>
                        </a>
                        <Tooltip title={`Copy ${HITS_GROUP_FIELD}`}>
                          <Button
                            size="small"
                            variant="text"
                            onClick={createCopyHandler(label)}
                            startIcon={<CopyIcon/>}
                            ariaLabel="copy row"
                          />
                        </Tooltip>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Modal>
      )}
    </>
  );
};

export default LegendHitsMenuOther;
