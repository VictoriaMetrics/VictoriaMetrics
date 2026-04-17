import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchItem } from "./hooks/useFetchItem";
import "./style.scss";
import { Group as APIGroup, Rule as APIRule } from "../../types";
import ItemHeader from "../../components/ExploreAlerts/ItemHeader";
import BaseRule from "../../components/ExploreAlerts/BaseRule";
import Modal from "../../components/Main/Modal/Modal";
import { getStates } from "../../components/ExploreAlerts/helpers";
import { useFetchGroup } from "./hooks/useFetchGroup";

interface ExploreRuleProps {
  groupId: string;
  id: string;
  mode: string;
  onClose: () => void;
}

const ExploreRule = ({ groupId, id, mode, onClose }: ExploreRuleProps) => {
  const {
    item,
    isLoading: isLoadingItem,
    error: errorItem,
  } = useFetchItem<APIRule>({ groupId, id, mode });

  const {
    group,
    isLoading: isLoadingGroup,
    error: errorGroup,
  } = useFetchGroup<APIGroup>({ id: groupId });

  const error =  errorItem || errorGroup;
  const isLoading = isLoadingItem || isLoadingGroup;

  if (isLoading) return (
    <Spinner />
  );

  if (error) return (
    <Alert variant="error">{error}</Alert>
  );

  const noItemFound = `No rule with group ID=${groupId}, rule ID=${id} found!`;

  return (
    <Modal
      className="vm-explore-alerts"
      title={item ? (
        <ItemHeader
          entity="rule"
          type={item.type}
          groupId={item.group_id}
          states={getStates(item)}
          id={item.id}
          name={item.name}
          onClose={onClose}
        />
      ) : "Rule not found"}
      onClose={onClose}
    >
      <div className="vm-explore-alerts">
        {item ? (
          <BaseRule
            item={item}
            group={group}
          />
        ) : (
          <Alert variant="info">{noItemFound}</Alert>
        )}
      </div>
    </Modal>
  );
};

export default ExploreRule;
