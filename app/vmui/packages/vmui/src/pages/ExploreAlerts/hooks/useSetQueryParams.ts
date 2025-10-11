import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface rulesQueryProps {
  types?: string;
  states?: string;
  search?: string;
  rule_id: string;
  group_id: string;
  alert_id: string;
}

export const useRulesSetQueryParams = ({
  types,
  states,
  search,
  rule_id,
  alert_id,
  group_id,
}: rulesQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      types,
      states,
      search,
      alert_id,
      rule_id,
      group_id,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [
    types,
    states,
    search,
    rule_id,
    group_id,
    alert_id,
  ]);
};

interface notifiersQueryProps {
  kinds: string;
  search: string;
}

export const useNotifiersSetQueryParams = ({
  kinds,
  search,
}: notifiersQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      kinds,
      search,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [kinds, search]);
};
