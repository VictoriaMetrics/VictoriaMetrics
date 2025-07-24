import { FC } from "preact/compat";
import RuleHeader from "../RuleHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import dayjs from "dayjs";
import { isDefaultDatasourceType } from "../../../constants/appType";
import { Link } from "react-router-dom";

interface RuleProps {
  state: string
  rule: APIRule
  expandRules: Set<string>
  onRulesChange: (a: boolean) => void
}

const Rule: FC<RuleProps> = ({
  state,
  rule,
  expandRules,
  onRulesChange,
}) => {

  const openLink = (link: string) => {
    return () => {
      window.open(link, "_blank");
    };
  };

  const queryLink = (datasourceType: string): string => {
    if (!isDefaultDatasourceType(datasourceType)) {
      return "";
    }
    return `/?g0.expr=${encodeURI(rule.query)}`;
  };

  return (
    <div
      className={`vm-explore-alerts-rule ${state.replace(" ", "-")}`}
    >
      <Accordion
        defaultExpanded={expandRules.has(rule.id)}
        onChange={onRulesChange}
        key={`rule-${rule.id}`}
        title={(
          <RuleHeader
            rule={rule}
          />
        )}
      >
        <div className="vm-explore-alerts-rule-item">
          <table>
            <tbody>
              <tr>
                <td className="vm-col-small">Query</td>
                <td>
                  {queryLink(rule.datasourceType) && (
                    <Link
                      to={queryLink(rule.datasourceType)}
                    >
                      <pre>
                        <code className="language-promql">{rule.query}</code>
                      </pre>
                    </Link>
                  ) || (
                    <pre>
                      <code className="language-promql">{rule.query}</code>
                    </pre>
                  )}
                </td>
              </tr>
              <tr>
                <td className="vm-col-small">Duration</td>
                <td>{dayjs.duration(rule.duration, "seconds").humanize()}</td>
              </tr>
              <tr>
                <td className="vm-col-small">Last evaluation</td>
                <td>{dayjs().to(dayjs(rule.lastEvaluation))}</td>
              </tr>
              {rule.lastError && (
                <tr>
                  <td className="vm-col-small">Last error</td>
                  <td>{rule.lastError}</td>
                </tr>
              )}
              {rule.labels && (
                <tr>
                  <td className="vm-col-small">Labels</td>
                  <td>
                    <div className="vm-badge-container">
                      {Object.entries(rule.labels).map(([name, value]) => (
                        <span
                          key={name}
                          className="vm-badge"
                        >{`${name}: ${value}`}</span>
                      ))}
                    </div>
                  </td>
                </tr>
              )}
              {rule.annotations && (
                <tr>
                  <td colSpan={2}>Annotations</td>
                </tr>
              )}
              {Object.entries(rule.annotations || {}).map(([name, value]) => (
                <tr key={name}>
                  <td className="vm-col-small">{name}</td>
                  <td>{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rule.alerts && (
            <>
              <span className="title">Alerts</span>
              <table>
                <thead>
                  <tr>
                    <th className="vm-col-small">Active since</th>
                    <th className="vm-col-small">Value</th>
                    <th>Labels</th>
                  </tr>
                </thead>
                <tbody>
                  {rule.alerts.map(alert => (
                    <tr
                      id={`alert-${alert.id}`}
                      key={alert.id}
                      onClick={openLink(alert.source)}
                    >
                      <td className="vm-col-small">{dayjs().to(dayjs(alert.activeAt))}</td>
                      <td className="vm-col-small">
                        <div className={`vm-badge ${alert.state}`}>{alert.value}</div>
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
      </Accordion>
    </div>
  );
};

export default Rule;
