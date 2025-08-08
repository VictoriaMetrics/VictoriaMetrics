import { useMemo } from "preact/compat";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchItem } from "./hooks/useFetchItem";
import "./style.scss";
import { Rule as APIRule } from "../../types";
import BaseRule from "../../components/ExploreAlerts/BaseRule";
import Modal from "../../components/Main/Modal/Modal";

interface ExploreRuleProps {
  groupId: string;
  id: string;
  mode: string;
  onClose: () => void;
}

const ExploreRule = ({ groupId, id, mode, onClose }: ExploreRuleProps) => {
  const {
    item,
    isLoading: loadingItem,
    error: errorItem,
  } = useFetchItem<APIRule>({ groupId, id, mode });

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

  const noItemFound = `No rule with group ID=${groupId}, rule ID=${id} found!`;

  return (
    <Modal
      title={item ? `Rule: ${item.name}` : "Rule not found"}
      onClose={onClose}
    >
      <div className="vm-explore-alerts">
        {item && (<BaseRule item={item} />) || (
          <Alert variant="info">{noItemFound}</Alert>
        )}
      </div>
    </Modal>
  );
};

export default ExploreRule;
