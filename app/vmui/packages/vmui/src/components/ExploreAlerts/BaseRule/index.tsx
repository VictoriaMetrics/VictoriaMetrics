import "./style.scss";
import { Rule as APIRule } from "../../../types";
import { useNavigate, createSearchParams } from "react-router-dom";
import { SearchIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import dayjs from "dayjs";

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
    window.open(`/#/?${createSearchParams(params).toString()}`, "_blank", "noopener noreferrer");
  };

  return (
    <div className="vm-explore-alerts-rule-item">
      <div></div>
      <table>
        <tbody>
          <tr className="align-end">
            <td colSpan={2}>
              <Button
                size="small"
                variant="outlined"
                color="gray"
                startIcon={<SearchIcon />}
                onClick={openQueryLink}
              >
                <span className="vm-button-text">Show query</span>
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
              <td>{dayjs.duration(item.duration, "seconds").format("H[h] m[m] s[s]")}</td>
            </tr>
          )}
          {!!item.lastEvaluation && (
            <tr>
              <td className="vm-col-md">Last evaluation</td>
              <td>{item.lastEvaluation}</td>
            </tr>
          )}
          {!!item.lastError && (
            <tr>
              <td className="vm-col-md">Last error</td>
              <td>{item.lastError}</td>
            </tr>
          )}
          {!!item?.labels?.length && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <div className="vm-badge-container">
                  {Object.entries(item.labels).map(([name, value]) => (
                    <span
                      key={name}
                      className="vm-badge"
                    >{`${name}: ${value}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {item.annotations && (
        <>
          <span className="title">Annotations</span>
          <table>
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
      {item.updates && (
        <>
          <span className="title">{`Last updates ${item.updates.length}/20`}</span>
          <table>
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
                  className="hoverable"
                >
                  <td className="vm-col-md">{update.time}</td>
                  <td className="vm-col-md">{update.samples}</td>
                  <td className="vm-col-md">{update.series_fetched}</td>
                  <td className="vm-col-md">{dayjs.duration(update.duration / 1e6).asSeconds().toFixed(3)}s</td>
                  <td className="vm-col-md">{update.at}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
      {item.alerts && (
        <>
          <span className="title">Alerts</span>
          <table>
            <thead>
              <tr>
                <th className="vm-col-sm">Active since</th>
                <th className="vm-col-sm">State</th>
                <th className="vm-col-sm">Value</th>
                <th>Labels</th>
              </tr>
            </thead>
            <tbody>
              {item.alerts.map((alert) => (
                <tr
                  id={`alert-${alert.id}`}
                  key={alert.id}
                  className="hoverable"
                  onClick={openAlertLink(alert.id)}
                >
                  <td className="vm-col-sm">
                    {dayjs().to(dayjs(alert.activeAt))}
                  </td>
                  <td className="vm-col-sm">
                    <div className="vm-badge">{alert.state}</div>
                  </td>
                  <td className="vm-col-sm">
                    <div className="vm-badge">{alert.value}</div>
                  </td>
                  <td className="vm-badge-container">
                    {Object.entries(alert.labels || {}).map(([name, value]) => (
                      <span
                        key={name}
                        className="vm-badge"
                      >{`${name}: ${value}`}</span>
                    ))}
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
