import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchItem } from "./hooks/useFetchItem";
import "./style.scss";
import { Alert as APIAlert } from "../../types";
import ItemHeader from "../../components/ExploreAlerts/ItemHeader";
import BaseAlert from "../../components/ExploreAlerts/BaseAlert";
import Modal from "../../components/Main/Modal/Modal";

interface ExploreAlertProps {
  groupId: string;
  id: string;
  mode: string;
  onClose: () => void;
}

const ExploreAlert = ({ groupId, id, mode, onClose }: ExploreAlertProps) => {
  const {
    item,
    isLoading,
    error,
  } = useFetchItem<APIAlert>({ groupId, id, mode });

  if (isLoading) return (
    <Spinner />
  );

  if (error) return (
    <Alert variant="error">{error}</Alert>
  );

  const noItemFound = `No alert with group ID=${groupId}, alert ID=${id} found!`;
  const states = {
    firing: 1,
  };

  return (
    <Modal
      className="vm-explore-alerts"
      title={item ? (
        <ItemHeader
          entity="alert"
          type="alerting"
          groupId={item.group_id}
          id={item.id}
          name={item.name}
          states={states}
          onClose={onClose}
        />
      ) : "Alert not found"}
      onClose={onClose}
    >
      <div className="vm-explore-alerts">
        {item && (<BaseAlert item={item} />) || (
          <Alert variant="info">{noItemFound}</Alert>
        )}
      </div>
    </Modal>
  );
};

export default ExploreAlert;
