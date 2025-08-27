import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchGroup } from "./hooks/useFetchGroup";
import "./style.scss";
import { Group as APIGroup } from "../../types";
import ItemHeader from "../../components/ExploreAlerts/ItemHeader";
import BaseGroup from "../../components/ExploreAlerts/BaseGroup";
import Modal from "../../components/Main/Modal/Modal";

interface ExploreGroupProps {
  id: string;
  onClose: () => void;
}

const ExploreGroup = ({ id, onClose }: ExploreGroupProps) => {
  const {
    group,
    isLoading,
    error,
  } = useFetchGroup<APIGroup>({ id });

  if (isLoading) return (
    <Spinner />
  );

  if (error) return (
    <Alert variant="error">{error}</Alert>
  );

  const noGroupFound = `No group ID=${id} found!`;

  return (
    <Modal
      className="vm-explore-alerts"
      title={group ? (
        <ItemHeader
          entity="group"
          groupId={id}
          name={group.name}
          states={group.states}
          onClose={onClose}
        />
      ) : "Rule not found"}
      onClose={onClose}
    >
      <div className="vm-explore-alerts">
        {group && (<BaseGroup group={group} />) || (
          <Alert variant="info">{noGroupFound}</Alert>
        )}
      </div>
    </Modal>
  );
};

export default ExploreGroup;
