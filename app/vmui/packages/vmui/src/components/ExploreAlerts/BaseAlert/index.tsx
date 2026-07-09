import { useMemo } from "preact/compat";
import "./style.scss";
import { Alert as APIAlert, Group } from "../../../types";
import { Link } from "react-router-dom";
import Button from "../../Main/Button/Button";
import Badges, { BadgeColor } from "../Badges";
import { formatEventTime } from "../helpers";
import {
  SearchIcon,
} from "../../Main/Icons";
import CodeExample from "../../Main/CodeExample/CodeExample";
import router from "../../../router";

interface BaseAlertProps {
  item: APIAlert;
  group?: Group;
}

const BaseAlert = ({ item, group }: BaseAlertProps) => {
  const query = item?.expression;
  const alertLabels = item?.labels || {};
  const alertLabelsItems = useMemo(() => {
    return Object.fromEntries(Object.entries(alertLabels).map(([name, value]) => [name, {
      color: "passive" as BadgeColor,
      value: value,
    }]));
  }, [alertLabels]);

  const queryLink = useMemo(() => {
    if (!group?.interval) return;

    const params = new URLSearchParams({
      "g0.expr": query,
      "g0.end_time": item.activeAt,
      // Interval is the Group's evaluation interval in float seconds as present in the file. See: /app/vmalert/rule/web.go
      "g0.step_input": `${group.interval}s`,
      "g0.relative_time": "none",
    });

    return `${router.home}?${params.toString()}`;
  }, [query, item.activeAt, group?.interval]);

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
              {queryLink && (
                <Link
                  to={queryLink}
                  target={"_blank"}
                  rel="noreferrer"
                >
                  <Button
                    size="small"
                    variant="outlined"
                    color="gray"
                    startIcon={<SearchIcon />}
                  >
                    <span className="vm-button-text">Run query</span>
                  </Button>
                </Link>
              )}
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
