import React, { FC } from "preact/compat";
import RuleHeader from "../RuleHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import dayjs from "dayjs";
import { highlight, languages } from "prismjs";
import "prismjs/components/prism-promql";
import parse from "html-react-parser";
import { IsDefaultDatasourceType } from "../../../constants/appType";

interface RuleProps {
  state: string
  rule: APIRule
  expandRules: string[]
  onRulesChange: (a: number) => void
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
    if (!IsDefaultDatasourceType(datasourceType)) {
      return "";
    }
    switch (datasourceType) {
      case "vlogs":
        return `./?#/?query=${encodeURI(rule.query)}`;
      default:
        return `./?#/?g0.expr=${encodeURI(rule.query)}`;
    }
  };

  return (
    <div
      className={`vm-explore-alerts-rule ${state.replace(" ", "-")}`}
    >
      <Accordion
        defaultExpanded={expandRules[rule.id]}
        onChange={onRulesChange(rule.id)}
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
                <td className="col-small">Query</td>
                <td>
                  {queryLink(rule.datasourceType) && (
                    <a
                      href={queryLink(rule.datasourceType)}
                    >
                      <pre>
                        <code className="language-promql">{parse(highlight(rule.query, languages.promql))}</code>
                      </pre>
                    </a>
                  ) || (
                    <pre>
                      <code className="language-promql">{parse(highlight(rule.query, languages.promql))}</code>
                    </pre>
                  )}
                </td>
              </tr>
              <tr>
                <td className="col-small">Duration</td>
                <td>{dayjs.duration(rule.duration, "seconds").humanize()}</td>
              </tr>
              <tr>
                <td className="col-small">Last evaluation</td>
                <td>{dayjs().to(dayjs(rule.lastEvaluation))}</td>
              </tr>
              {rule.lastError && (
                <tr>
                  <td className="col-small">Last error</td>
                  <td>{rule.lastError}</td>
                </tr>
              )}
              {rule.labels && (
                <tr>
                  <td className="col-small">Labels</td>
                  <td>
                    <div className="badge-container">
                      {Object.entries(rule.labels).map(([name, value]) => (
                        <span
                          key={name}
                          className="badge"
                        >{`${name}: ${value}`}</span>
                      ))}
                    </div>
                  </td>
                </tr>
              )}
              {rule.annotations && (
                <tr>
                  <td colSpan="2">Annotations</td>
                </tr>
              )}
              {Object.entries(rule.annotations || {}).map(([name, value]) => (
                <tr key={name}>
                  <td className="col-small">{name}</td>
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
                    <th className="col-small">Active since</th>
                    <th className="col-small">Value</th>
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
                      <td className="col-small">{dayjs().to(dayjs(alert.activeAt))}</td>
                      <td className="col-small">
                        <div className={`badge ${alert.state}`}>{alert.value}</div>
                      </td>
                      <td className="badge-container">
                        {Object.entries(alert.labels || {}).map(([name, value]) => (
                          <span
                            key={name}
                            className="badge"
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
