import React, { FC } from "preact/compat";
import "./style.scss";
import TextField from "../../components/Main/TextField/TextField";
import { useState } from "react";
import Button from "../../components/Main/Button/Button";
import { PlayIcon } from "../../components/Main/Icons";
import WithTemplateTutorial from "./WithTemplateTutorial/WithTemplateTutorial";

const WithTemplate: FC = () => {
  const [expr, setExpr] = useState("");
  const [result, setResult] = useState("");

  const handleChangeInput = (val: string) => {
    setExpr(val);
  };

  const handleRunQuery = () => {
    console.log("run");
  };

  return (
    <section className="vm-with-template">
      <div className="vm-with-template-body vm-block">
        <div className="vm-with-template-body__expr">
          <TextField
            type="textarea"
            label="MetricsQL query with optional WITH expressions"
            value={expr}
            autofocus
            onChange={handleChangeInput}
          />
        </div>
        <div className="vm-with-template-body__result">
          <TextField
            type="textarea"
            label="MetricsQL query after expanding WITH expressions and applying other optimizations"
            value={result}
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
