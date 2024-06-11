import React, { useEffect } from "preact/compat";
import { FunctionIcon } from "../components/Main/Icons";
import { AutocompleteOptions } from "../components/Main/Autocomplete/Autocomplete";
import { marked } from "marked";
import MetricsQL from "../assets/MetricsQL.md";
import { useQueryDispatch, useQueryState } from "../state/query/QueryStateContext";

const CATEGORY_TAG = "h3";
const FUNCTION_TAG = "h4";
const DESCRIPTION_TAG = "p";

const docsUrl = "https://docs.victoriametrics.com/MetricsQL.html";
const classLink = "vm-link vm-link_colored";

const prepareDescription = (text: string): string => {
  const replaceValue = `$1 target="_blank" class="${classLink}" $2${docsUrl}#`;
  return text.replace(/(<a) (href=")#/gm, replaceValue);
};

const getParagraph = (el: Element): Element[] => {
  const paragraphs: Element[] = [];
  let nextEl = el.nextElementSibling;
  while (nextEl && nextEl.tagName.toLowerCase() === DESCRIPTION_TAG) {
    if (nextEl) paragraphs.push(nextEl);
    nextEl = nextEl.nextElementSibling;
  }
  return paragraphs;
};

const createAutocompleteOption = (type: string, group: Element): AutocompleteOptions => {
  const value = group.textContent ?? "";
  const paragraphs = getParagraph(group);
  const description = paragraphs.map(p => p.outerHTML ?? "").join("\n");
  return {
    type,
    value,
    description: prepareDescription(description),
    icon: <FunctionIcon />,
  };
};

const processGroups = (groups: NodeListOf<Element>): AutocompleteOptions[] => {
  let type = "";
  return Array.from(groups).map(group => {
    const isCategory = group.tagName.toLowerCase() === CATEGORY_TAG;
    type = isCategory ? group.textContent ?? "" : type;
    return isCategory ? null : createAutocompleteOption(type, group);
  }).filter(Boolean) as AutocompleteOptions[];
};

const useGetMetricsQL = () => {
  const { metricsQLFunctions } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const processMarkdown = (text: string) => {
    const div = document.createElement("div");
    div.innerHTML = marked(text) as string;
    const groups = div.querySelectorAll(`${CATEGORY_TAG}, ${FUNCTION_TAG}`);
    return processGroups(groups);
  };

  useEffect(() => {
    const fetchMarkdown = async () => {
      try {
        const resp = await fetch(MetricsQL);
        const text = await resp.text();
        const result = processMarkdown(text);
        queryDispatch({ type: "SET_METRICSQL_FUNCTIONS", payload: result });
      } catch (e) {
        console.error("Error fetching or processing the MetricsQL.md file:", e);
      }
    };

    if (metricsQLFunctions.length) return;
    fetchMarkdown();
  }, []);

  return metricsQLFunctions;
};

export default useGetMetricsQL;
