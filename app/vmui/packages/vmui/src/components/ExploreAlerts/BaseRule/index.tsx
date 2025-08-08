import "./style.scss";
import { Rule as APIRule } from "../../../types";
import { useNavigate, createSearchParams } from "react-router-dom";
import { SearchIcon, DetailsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Alert from "../../Main/Alert/Alert";
import Badges, { BadgeColor } from "../Badges";
import dayjs from "dayjs";
import { formatDuration } from "../helpers";

interface BaseRuleProps {
  item: APIRule;
}

const BaseRule = ({ item }: BaseRuleProps) => {
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

  const openQueryLink = () => {
    const params = {
      "g0.expr": query,
      "g0.end_time": ""
    };
    window.open(`#/?${createSearchParams(params).toString()}`, "_blank", "noopener noreferrer");
  };

  return (
    <div className="vm-explore-alerts-rule-item">
      <div></div>
      <table>
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
            <td className="vm-col-md">Query</td>
            <td>
              <pre>
                <code className="language-promql">{query}</code>
              </pre>
            </td>
          </tr>
          {!!item.duration && (
            <tr>
              <td className="vm-col-md">For</td>
              <td>{formatDuration(item.duration)}</td>
            </tr>
          )}
          {!!item.lastEvaluation && (
            <tr>
              <td className="vm-col-md">Last evaluation</td>
              <td>{dayjs(item.lastEvaluation).format("DD MMM YYYY HH:mm:ss")}</td>
            </tr>
          )}
          {!!item.lastError && item.health !== "ok" && (
            <tr>
              <td className="vm-col-md">Last error</td>
              <td>
                <Alert variant="error">{item.lastError}</Alert>
              </td>
            </tr>
          )}
          {!!Object.keys(item?.labels || {}).length && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <Badges
                  items={Object.fromEntries(Object.entries(item.labels).map(([name, value]) => [name, {
                    color: "passive",
                    value: value,
                  }]))}
                />
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {!!Object.keys(item?.annotations || {}).length && (
        <>
          <span className="title">Annotations</span>
          <table className="fixed">
            <tbody>
              {Object.entries(item.annotations || {}).map(([name, value]) => (
                <tr key={name}>
                  <td className="vm-col-md">{name}</td>
                  <td>{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
      {!!item?.updates?.length && (
        <>
          <span className="title">{`Last updates ${item.updates.length}/${item.max_updates_entries}`}</span>
          <table className="fixed">
            <thead>
              <tr>
                <th className="vm-col-md">Updated at</th>
                <th className="vm-col-md">Series returned</th>
                <th className="vm-col-md">Series fetched</th>
                <th className="vm-col-md">Duration</th>
                <th className="vm-col-md">Executed at</th>
              </tr>
            </thead>
            <tbody>
              {item.updates.map((update) => (
                <tr
                  key={update.at}
                >
                  <td className="vm-col-md">{dayjs(update.time).format("DD MMM YYYY HH:mm:ss")}</td>
                  <td className="vm-col-md">{update.samples}</td>
                  <td className="vm-col-md">{update.series_fetched}</td>
                  <td className="vm-col-md">{formatDuration(update.duration / 1e9)}</td>
                  <td className="vm-col-md">{dayjs(update.at).format("DD MMM YYYY HH:mm:ss")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
      {!!item?.alerts?.length && (
        <>
          <span className="title">Alerts</span>
          <table className="fixed">
            <thead>
              <tr>
                <th className="vm-col-sm">Active since</th>
                <th className="vm-col-sm">State</th>
                <th className="vm-col-sm">Value</th>
                <th>Labels</th>
                <th className="vm-col-hidden"></th>
              </tr>
            </thead>
            <tbody>
              {item.alerts.map((alert) => (
                <tr
                  id={`alert-${alert.id}`}
                  key={alert.id}
                >
                  <td className="vm-col-sm">
                    {dayjs(alert.activeAt).format("DD MMM YYYY HH:mm:ss")}
                  </td>
                  <td className="vm-col-sm">
                    <Badges
                      items={{ [alert.state]: { color: alert.state as BadgeColor } }}
                    />
                  </td>
                  <td className="vm-col-sm">
                    <Badges
                      items={{ [alert.value]: { color: "passive" } }}
                    />
                  </td>
                  <td>
                    <Badges
                      align="center"
                      items={Object.fromEntries(Object.entries(alert.labels || {}).map(([name, value]) => [name, {
                        color: "passive",
                        value: value,
                      }]))}
                    />
                  </td>
                  <td className="vm-col-hidden">
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
