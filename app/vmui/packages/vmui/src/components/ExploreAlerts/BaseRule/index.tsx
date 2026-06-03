import { useMemo } from "preact/compat";
import "./style.scss";
import { Group, Rule as APIRule } from "../../../types";
import { useNavigate, Link } from "react-router-dom";
import { SearchIcon, DetailsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Alert from "../../Main/Alert/Alert";
import Badges, { BadgeColor } from "../Badges";
import { formatDuration, formatEventTime } from "../helpers";
import CodeExample from "../../Main/CodeExample/CodeExample";
import router from "../../../router";

interface BaseRuleProps {
  item: APIRule;
  group?: Group;
}

const BaseRule = ({ item, group }: BaseRuleProps) => {
  const query = item?.query;
  const navigate = useNavigate();
  const openAlertLink = (id: string) => {
    return () => {
      navigate({
        pathname: "/rules",
        search:   `group_id=${item.group_id}&alert_id=${id}`,
      });
    };
  };

  const ruleLabels = item?.labels || {};
  const ruleLabelsItems = useMemo(() => {
    return Object.fromEntries(Object.entries(ruleLabels).map(([name, value]) => [name, {
      color: "passive" as BadgeColor,
      value: value,
    }]));
  }, [ruleLabels]);

  const queryLink = useMemo(() => {
    if (!group?.interval) return;

    const params = new URLSearchParams({
      "g0.expr": query,
      "g0.end_time": item.lastEvaluation,
      // Interval is the Group's evaluation interval in float seconds as present in the file. See: /app/vmalert/rule/web.go
      "g0.step_input": `${group.interval}s`,
      "g0.relative_time": "none",
    });

    return `${router.home}?${params.toString()}`;
  }, [query, item.lastEvaluation, group?.interval]);

  return (
    <div className="vm-explore-alerts-rule-item">
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
          {!!item.duration && (
            <tr>
              <td>For</td>
              <td>{formatDuration(item.duration)}</td>
            </tr>
          )}
          <tr>
            <td>Last evaluation</td>
            <td>{formatEventTime(item.lastEvaluation)}</td>
          </tr>
          {!!item.lastError && item.health !== "ok" && (
            <tr>
              <td>Last error</td>
              <td>
                <Alert variant="error">{item.lastError}</Alert>
              </td>
            </tr>
          )}
          {!!Object.keys(ruleLabelsItems).length && (
            <tr>
              <td>Labels</td>
              <td>
                <Badges
                  items={ruleLabelsItems}
                />
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {!!Object.keys(item?.annotations || {}).length && (
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
      {!!item?.updates?.length && (
        <>
          <span className="vm-alerts-title">{`Last updates ${item.updates.length}/${item.max_updates_entries}`}</span>
          <table>
            <thead>
              <tr>
                <th>Updated at</th>
                <th>Series returned</th>
                <th>Series fetched</th>
                <th>Duration</th>
                <th>Execution timestamp</th>
              </tr>
            </thead>
            <tbody>
              {item.updates.map((update) => (
                <tr
                  key={update.at}
                >
                  <td>{formatEventTime(update.time)}</td>
                  <td>{update.samples}</td>
                  <td>{update.series_fetched}</td>
                  <td>{formatDuration(update.duration / 1e9)}</td>
                  <td>{formatEventTime(update.at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
      {!!item?.alerts?.length && (
        <>
          <span className="vm-alerts-title">Alerts</span>
          <table className="vm-alerts-table">
            <colgroup>
              <col className="vm-col-sm"/>
              <col className="vm-col-sm"/>
              <col className="vm-col-sm"/>
              <col/>
              <col className="vm-col-hidden"/>
            </colgroup>
            <thead>
              <tr>
                <th>Active since</th>
                <th>State</th>
                <th>Value</th>
                <th className="vm-alerts-title">Labels</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {item.alerts.map((alert) => (
                <tr
                  id={`alert-${alert.id}`}
                  key={alert.id}
                >
                  <td>{formatEventTime(alert.activeAt)}</td>
                  <td>
                    <Badges
                      items={{ [alert.state]: { color: alert.state as BadgeColor } }}
                    />
                  </td>
                  <td>
                    <Badges
                      items={{ [alert.value]: { color: "passive" } }}
                    />
                  </td>
                  <td>
                    <Badges
                      align="start"
                      items={Object.fromEntries(Object.entries(alert.labels || {}).map(([name, value]) => [name, {
                        color: "passive",
                        value: value,
                      }]))}
                    />
                  </td>
                  <td>
                    <Button
                      className="vm-button-borderless"
                      size="small"
                      variant="outlined"
                      color="gray"
                      startIcon={<DetailsIcon />}
                      onClick={openAlertLink(alert.id)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
};

export default BaseRule;
