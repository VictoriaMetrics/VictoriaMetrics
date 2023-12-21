import React, { FC, useMemo } from "preact/compat";
import { DataAnalyzerType } from "../index";
import Button from "../../../components/Main/Button/Button";
import { InfoIcon } from "../../../components/Main/Icons";
import useBoolean from "../../../hooks/useBoolean";
import Modal from "../../../components/Main/Modal/Modal";
import "./style.scss";

type Props = {
  data: DataAnalyzerType[]
}

const QueryAnalyzerInfo: FC<Props> = ({ data }) => {
  const dataWithStats = useMemo(() => data.filter(d => d.stats && d.data.resultType === "matrix"), [data]);
  const comment = useMemo(() => data.find(d => d?.vmui?.comment)?.vmui?.comment, [data]);

  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  return (
    <>
      <div>
        <Button
          startIcon={<InfoIcon/>}
          variant="outlined"
          color="warning"
          onClick={handleOpenModal}
        >
          Show report info
        </Button>
      </div>

      {openModal && (
        <Modal
          title="Report info"
          onClose={handleCloseModal}
        >
          <div className="vm-query-analyzer-info">
            {comment && (
              <div className="vm-query-analyzer-info-item vm-query-analyzer-info-item_comment">
                <div className="vm-query-analyzer-info-item__title">Comment:</div>
                <div className="vm-query-analyzer-info-item__text">{comment}</div>
              </div>
            )}
            {dataWithStats.map((d, i) => (
              <div
                className="vm-query-analyzer-info-item"
                key={i}
              >
                <div className="vm-query-analyzer-info-item__title">
                  {dataWithStats.length > 1 ? `Query ${i + 1}:` : "Stats:"}
                </div>
                <div className="vm-query-analyzer-info-item__text">
                  {Object.entries(d.stats || {}).map(([key, value]) => (
                    <div key={key}>
                      {key}: {value ?? "-"}
                    </div>
                  ))}
                  isPartial: {String(d.isPartial ?? "-")}
                </div>
              </div>
            ))}
            <div className="vm-query-analyzer-info-type">
              {dataWithStats[0]?.vmui?.params ? "The report was created using vmui" : "The report was created manually"}
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default QueryAnalyzerInfo;
