import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface rulesQueryProps {
  ruleTypes?: string
  states?: string
  search?: string
}

export const useRulesSetQueryParams = ({ ruleTypes, states, search }: rulesQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ruleTypes,
      states,
      search,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [ruleTypes, states, search]);
  useEffect(setSearchParamsFromState, []);
};

interface notifiersQueryProps {
  kinds: string
  search: string
}

export const useNotifiersSetQueryParams = ({ kinds, search }: notifiersQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      kinds,
      search,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [kinds, search]);
  useEffect(setSearchParamsFromState, []);
};
