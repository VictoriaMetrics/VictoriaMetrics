import React, { FC } from "preact/compat";
import "./style.scss";
import TextField from "../../components/Main/TextField/TextField";
import { useEffect, useState } from "react";
import Button from "../../components/Main/Button/Button";
import { PlayIcon } from "../../components/Main/Icons";
import WithTemplateTutorial from "./WithTemplateTutorial/WithTemplateTutorial";
import { useExpandWithExprs } from "./hooks/useExpandWithExprs";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useSearchParams } from "react-router-dom";

const WithTemplate: FC = () => {
  const [searchParams] = useSearchParams();

  const { data, loading, error, expand } = useExpandWithExprs();
  const [expr, setExpr] = useState(searchParams.get("expr") || "");

  const handleChangeInput = (val: string) => {
    setExpr(val);
  };

  const handleRunQuery = () => {
    expand(expr);
  };

  useEffect(() => {
    if (expr) expand(expr);
  }, []);

  return (
    <section className="vm-with-template">
      {loading && <Spinner />}

      <div className="vm-with-template-body vm-block">
        <div className="vm-with-template-body__expr">
          <TextField
            type="textarea"
            label="MetricsQL query with optional WITH expressions"
            value={expr}
            error={error}
            autofocus
            onEnter={handleRunQuery}
            onChange={handleChangeInput}
          />
        </div>
        <div className="vm-with-template-body__result">
          <TextField
            type="textarea"
            label="MetricsQL query after expanding WITH expressions and applying other optimizations"
            value={data}
            disabled
          />
        </div>
        <div className="vm-with-template-body-top">
          <Button
            variant="contained"
            onClick={handleRunQuery}
            startIcon={<PlayIcon/>}
          >
            Expand
          </Button>
        </div>
      </div>
      <div className="vm-block">
        <WithTemplateTutorial/>
      </div>
    </section>
  );
};

export default WithTemplate;
