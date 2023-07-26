import React, { FC, useCallback, useEffect } from "preact/compat";
import "./style.scss";
import Button from "../../components/Main/Button/Button";
import { InfoIcon, PlayIcon, WikiIcon } from "../../components/Main/Icons";
import "./style.scss";
import { useRelabelDebug } from "./hooks/useRelabelDebug";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useSearchParams } from "react-router-dom";
import useStateSearchParams from "../../hooks/useStateSearchParams";
import TextField from "../../components/Main/TextField/TextField";

const example = {
  config: `- if: '{bar_label=~"b.*"}'
  source_labels: [foo_label, bar_label]
  separator: "_"
  target_label: foobar
- action: labeldrop
  regex: "foo_.*"
- target_label: job
  replacement: "my-application-2"`,
  labels: "{__name__=\"my_metric\", bar_label=\"bar\", foo_label=\"foo\", job=\"my-application\", instance=\"192.168.0.1\"}"
};

const Relabel: FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();

  const { data, loading, error, fetchData } = useRelabelDebug();

  const [config, setConfig] = useStateSearchParams("", "config");
  const [labels, setLabels] = useStateSearchParams("", "labels");

  const handleChangeConfig = (val?: string) => {
    setConfig(val || "");
  };

  const handleChangeLabels = (val?: string) => {
    setLabels(val || "");
  };

  const handleRunQuery = useCallback(() => {
    fetchData(config, labels);
    searchParams.set("config", config);
    searchParams.set("labels", labels);
    setSearchParams(searchParams);
  }, [config, labels]);

  const handleRunExample = () => {
    const { config, labels } = example;
    setConfig(config);
    setLabels(labels);
    fetchData(config, labels);
    searchParams.set("config", config);
    searchParams.set("labels", labels);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const queryConfig = searchParams.get("config") || "";
    const queryLabels = searchParams.get("labels") || "";
    if (queryLabels || queryConfig) {
      fetchData(queryConfig, queryLabels);
      setConfig(queryConfig);
      setLabels(queryLabels);
    }
  }, []);

  return (
    <section className="vm-relabeling">
      {loading && <Spinner/>}
      <div className="vm-relabeling-header vm-block">
        <div className="vm-relabeling-header-configs">
          <TextField
            type="textarea"
            label="Relabel configs"
            value={config}
            autofocus
            onChange={handleChangeConfig}
            onEnter={handleRunQuery}
          />
        </div>
        <div className="vm-relabeling-header__labels">
          <TextField
            type="textarea"
            label="Labels"
            value={labels}
            onChange={handleChangeLabels}
            onEnter={handleRunQuery}
          />
        </div>
        <div className="vm-relabeling-header-bottom">
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/relabeling.html"
            rel="help noreferrer"
          >
            <InfoIcon/>
            Relabeling cookbook
          </a>
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/vmagent.html#relabeling"
            rel="help noreferrer"
          >
            <WikiIcon/>
            Documentation
          </a>
          <Button
            variant="text"
            onClick={handleRunExample}
          >
            Try example
          </Button>
          <Button
            variant="contained"
            onClick={handleRunQuery}
            startIcon={<PlayIcon/>}
          >
            Submit
          </Button>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {data && (
        <div className="vm-relabeling-steps vm-block">
          {data.originalLabels && (
            <div className="vm-relabeling-steps-item">
              <div className="vm-relabeling-steps-item__row">
                <span>Original labels:</span>
                <code dangerouslySetInnerHTML={{ __html: data.originalLabels }}/>
              </div>
            </div>
          )}

          {data.steps.map((step, index) => (
            <div
              className="vm-relabeling-steps-item"
              key={index}
            >
              <div className="vm-relabeling-steps-item__row">
                <span>Step:</span>
                {index + 1}
              </div>
              <div className="vm-relabeling-steps-item__row">
                <span>Relabeling Rule:</span>
                <code>
                  <pre>{step.rule}</pre>
                </code>
              </div>
              <div className="vm-relabeling-steps-item__row">
                <span>Input Labels:</span>
                <code>
                  <pre dangerouslySetInnerHTML={{ __html: step.inLabels }}/>
                </code>
              </div>
              <div className="vm-relabeling-steps-item__row">
                <span>Output labels:</span>
                <code>
                  <pre dangerouslySetInnerHTML={{ __html: step.outLabels }}/>
                </code>
              </div>
            </div>
          ))}

          {data.resultingLabels && (
            <div className="vm-relabeling-steps-item">
              <div className="vm-relabeling-steps-item__row">
                <span>Resulting labels:</span>
                <code dangerouslySetInnerHTML={{ __html: data.resultingLabels }}/>
              </div>
            </div>
          )}
        </div>
      )}
    </section>
  );
};

export default Relabel;
