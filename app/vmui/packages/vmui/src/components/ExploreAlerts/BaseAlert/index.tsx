import { useMemo } from "preact/compat";
import "./style.scss";
import { Alert as APIAlert } from "../../../types";
import { createSearchParams } from "react-router-dom";
import Button from "../../Main/Button/Button";
import Badges, { BadgeColor } from "../Badges";
import { formatEventTime } from "../helpers";
import {
  SearchIcon,
} from "../../Main/Icons";
import CodeExample from "../../Main/CodeExample/CodeExample";

interface BaseAlertProps {
  item: APIAlert;
}

const BaseAlert = ({ item }: BaseAlertProps) => {
  const query = item?.expression;
  const alertLabels = item?.labels || {};
  const alertLabelsItems = useMemo(() => {
    return Object.fromEntries(Object.entries(alertLabels).map(([name, value]) => [name, {
      color: "passive" as BadgeColor,
      value: value,
    }]));
  }, [alertLabels]);

  const openQueryLink = () => {
    const params = {
      "g0.expr": query,
      "g0.end_time": ""
    };
    window.open(`#/?${createSearchParams(params).toString()}`, "_blank", "noopener noreferrer");
  };

  return (
    <div className="vm-explore-alerts-alert-item">
      <table>
        <colgroup>
          <col className="vm-col-md"/>
          <col/>
        </colgroup>
        <tbody>
          <tr>
            <td
              style={{ "text-align": "end" }}
              colSpan={2}
            >
              <Button
                size="small"
                variant="outlined"
                color="gray"
                startIcon={<SearchIcon />}
                onClick={openQueryLink}
              >
                <span className="vm-button-text">Run query</span>
              </Button>
            </td>
          </tr>
          <tr>
            <td>Query</td>
            <td>
              <CodeExample
                code={query}
              />
            </td>
          </tr>
          <tr>
            <td>Active at</td>
            <td>{formatEventTime(item.activeAt)}</td>
          </tr>
          {!!Object.keys(alertLabels).length && (
            <tr>
              <td>Labels</td>
              <td>
                <Badges
                  items={alertLabelsItems}
                />
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {!!Object.keys(item.annotations || {}).length && (
        <>
          <span className="vm-alerts-title">Annotations</span>
          <table>
            <colgroup>
              <col className="vm-col-md"/>
              <col/>
            </colgroup>
            <tbody>
              {Object.entries(item.annotations || {}).map(([name, value]) => (
                <tr key={name}>
                  <td>{name}</td>
                  <td>{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
};

export default BaseAlert;
