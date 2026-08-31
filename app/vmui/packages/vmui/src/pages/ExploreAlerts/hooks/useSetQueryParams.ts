import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface rulesQueryProps {
  type?: string;
  states?: string;
  search?: string;
  vmalert_source?: string;
  rule_id: string;
  group_id: string;
  alert_id: string;
}

export const useRulesSetQueryParams = ({
  type,
  states,
  search,
  vmalert_source,
  rule_id,
  alert_id,
  group_id,
}: rulesQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      type,
      states,
      search,
      alert_id,
      rule_id,
      group_id,
    });

    // vmalert_source is passed outside of compactObject on purpose. compactObject drops
    // empty values, and setSearchParamsFromKeys only deletes a key when it is present and
    // empty - so a cleared source filter would leave a stale vmalert_source in the URL,
    // which would silently come back as a filter on the next page load.
    setSearchParamsFromKeys({ ...params, vmalert_source: vmalert_source || "" });
  };

  useEffect(setSearchParamsFromState, [
    type,
    states,
    search,
    vmalert_source,
    rule_id,
    group_id,
    alert_id,
  ]);
};

interface notifiersQueryProps {
  kinds: string;
  search: string;
  vmalert_source?: string;
}

export const useNotifiersSetQueryParams = ({
  kinds,
  search,
  vmalert_source,
}: notifiersQueryProps) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      kinds,
      search,
    });

    // vmalert_source is passed outside of compactObject - see the comment
    // at useRulesSetQueryParams.
    setSearchParamsFromKeys({ ...params, vmalert_source: vmalert_source || "" });
  };

  useEffect(setSearchParamsFromState, [kinds, search, vmalert_source]);
};
