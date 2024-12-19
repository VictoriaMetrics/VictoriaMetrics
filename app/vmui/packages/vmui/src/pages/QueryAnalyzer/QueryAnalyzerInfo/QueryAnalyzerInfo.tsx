import React, { FC, useMemo } from "preact/compat";
import { DataAnalyzerType } from "../index";
import {
  ClockIcon,
  CommentIcon,
  InfoIcon,
  TimelineIcon
} from "../../../components/Main/Icons";
import { TimeParams } from "../../../types";
import "./style.scss";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import useBoolean from "../../../hooks/useBoolean";
import Modal from "../../../components/Main/Modal/Modal";
import { marked } from "marked";
import Button from "../../../components/Main/Button/Button";
import get from "lodash.get";

type Props = {
  data: DataAnalyzerType[];
  period?: TimeParams;
}

const QueryAnalyzerInfo: FC<Props> = ({ data, period }) => {
  const dataWithStats = useMemo(() => data.filter(d => d.vmui || d.stats), [data]);
  const title = dataWithStats.find(d => d?.vmui?.title)?.vmui?.title || "Report";
  const comment = dataWithStats.find(d => d?.vmui?.comment)?.vmui?.comment;

  const table = useMemo(() => {
    return [
      "vmui.endpoint",
      ...new Set(dataWithStats.flatMap(d => [
        ...Object.keys(d.vmui?.params || []).map(key => `vmui.params.${key}`),
        ...Object.keys(d.stats || []).map(key => `stats.${key}`),
        "isPartial"
      ]))
    ].map(key => ({
      column: key.split(".").pop(),
      values: dataWithStats.map(data => get(data, key, "-"))
    })).filter(({ values }) => values.length && values.every(v => v !== "-"));
  }, [dataWithStats]);

  const timeRange = useMemo(() => {
    if (!period) return "";
    const start = dayjs(period.start * 1000).tz().format(DATE_TIME_FORMAT);
    const end = dayjs(period.end * 1000).tz().format(DATE_TIME_FORMAT);
    return `${start} - ${end}`;
  }, [period]);

  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  return (
    <>
      <div className="vm-query-analyzer-info-header">
        <h1 className="vm-query-analyzer-info-header__title">{title}</h1>
        {timeRange && (
          <div className="vm-query-analyzer-info-header__timerange">
            <ClockIcon/> {timeRange}
          </div>
        )}
        {period?.step && (
          <div className="vm-query-analyzer-info-header__timerange">
            <TimelineIcon/> step {period.step}
          </div>
        )}
        {(comment || !!table.length) && (
          <div className="vm-query-analyzer-info-header__info">
            <Button
              startIcon={<InfoIcon/>}
              variant="outlined"
              color="warning"
              onClick={handleOpenModal}
            >
              Show stats{comment && " & comments"}
            </Button>
          </div>
        )}
      </div>
      {openModal && (
        <Modal
          title={title}
          onClose={handleCloseModal}
        >
          <div className="vm-query-analyzer-info__modal">
            {!!table.length && (
              <div className="vm-query-analyzer-info-stats">
                <div className="vm-query-analyzer-info-comment-header">
                  <InfoIcon/>
                  Stats
                </div>
                <table>
                  <thead>
                    <tr>
                      {table.map(({ column }) => (
                        <th key={column}>
                          {column}
                        </th>
                      ))}
                    </tr>
                  </thead>

                  <tbody>
                    {table[0]?.values.map((_, rowIndex) => (
                      <tr key={rowIndex}>
                        {table.map(({ values }, j) => (
                          <td key={j}>
                            {values[rowIndex]}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {comment && (
              <div className="vm-query-analyzer-info-comment">
                <div className="vm-query-analyzer-info-comment-header">
                  <CommentIcon/>
                  Comments
                </div>
                <div
                  className="vm-query-analyzer-info-comment-body vm-markdown"
                  dangerouslySetInnerHTML={{ __html: (marked(comment) as string) || comment }}
                />
              </div>
            )}
          </div>
        </Modal>
      )}
    </>
  );
};

export default QueryAnalyzerInfo;
