import { FC } from "preact/compat";
import "./style.scss";
import Button from "../../Main/Button/Button";
import { TipIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Modal from "../../Main/Modal/Modal";
import useBoolean from "../../../hooks/useBoolean";
import tips from "./constants/tips";

const GraphTips: FC = () => {
  const {
    value: showTips,
    setFalse: handleCloseTips,
    setTrue: handleOpenTips
  } = useBoolean(false);

  return (
    <>
      <Tooltip title={"Show tips on working with the graph"}>
        <Button
          variant="text"
          color={"gray"}
          startIcon={<TipIcon/>}
          onClick={handleOpenTips}
          ariaLabel="open the tips"
        />
      </Tooltip>
      {showTips && (
        <Modal
          title={"Tips on working with the graph and the legend"}
          onClose={handleCloseTips}
        >
          <div className="fc-graph-tips">
            {tips.map(({ title, description }) => (
              <div
                className="fc-graph-tips-item"
                key={title}
              >
                <h4 className="fc-graph-tips-item__action">
                  {title}
                </h4>
                <p className="fc-graph-tips-item__description">
                  {description}
                </p>
              </div>
            ))}
          </div>
        </Modal>
      )}
    </>
  );
};

export default GraphTips;
