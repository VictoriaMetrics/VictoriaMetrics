import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface rulesQueryProps {
  rule_types: string
  states: string
}

export const useRulesSetQueryParams = ({ rule_types, states }: rulesQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      rule_types,
      states,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [rule_types, states]);
  useEffect(setSearchParamsFromState, []);
};

interface notifiersQueryProps {
  kinds: string
}

export const useNotifiersSetQueryParams = ({ kinds }: notifiersQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      kinds,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [kinds]);
  useEffect(setSearchParamsFromState, []);
};
