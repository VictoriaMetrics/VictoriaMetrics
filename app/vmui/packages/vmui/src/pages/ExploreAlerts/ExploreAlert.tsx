import { useMemo } from "preact/compat";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchItem } from "./hooks/useFetchItem";
import "./style.scss";
import { Alert as APIAlert } from "../../types";
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
    isLoading: loadingItem,
    error: errorItem,
  } = useFetchItem<APIAlert>({ groupId, id, mode });

  const isLoading = useMemo(() => {
    return loadingItem;
  }, [loadingItem]);

  const error = useMemo(() => {
    return errorItem;
  }, [errorItem]);

  if (isLoading) return (
    <Spinner />
  );

  if (error) return (
    <Alert variant="error">{error}</Alert>
  );

  const noItemFound = `No alert with group ID=${groupId}, alert ID=${id} found!`;

  return (
    <Modal
      title={item ? `Alert: ${item.name}` : "Alert not found"}
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
