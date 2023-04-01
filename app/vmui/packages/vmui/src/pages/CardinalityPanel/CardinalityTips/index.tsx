import { TipIcon } from "../../../components/Main/Icons";
import React, { FC } from "preact/compat";
import { ReactNode } from "react";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";
import "./style.scss";

const TipCard: FC<{ title?: string, children: ReactNode }> = ({ title, children }) => (
  <div className="vm-cardinality-tip">
    <div className="vm-cardinality-tip-header">
      <div className="vm-cardinality-tip-header__tip-icon"><TipIcon/></div>
      <h4 className="vm-cardinality-tip-header__title">{title || "Tips"}</h4>
    </div>
    <p className="vm-cardinality-tip__description">
      {children}
    </p>
  </div>
);

export const TipHighNumberOfSeries: FC = () => (
  <TipCard title="Metrics with a high number of series">
    <ul>
      <li>
        Identify and eliminate labels with frequently changed values to reduce their&nbsp;
        <Hyperlink href='https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality'>cardinality</Hyperlink>
        &nbsp;and&nbsp;
        <Hyperlink href='https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate'>high churn rate</Hyperlink>
      </li>
      <li>
        Find unused time series and&nbsp;
        <Hyperlink href='https://docs.victoriametrics.com/relabeling.html'>drop entire metrics</Hyperlink>
      </li>
      <li>
        Aggregate time series before they got ingested into the database via&nbsp;
        <Hyperlink href='https://docs.victoriametrics.com/stream-aggregation.html'>streaming aggregation</Hyperlink>
      </li>
    </ul>
  </TipCard>
);

export const TipHighNumberOfValues: FC = () => (
  <TipCard title="Labels with a high number of unique values">
    <ul>
      <li>Decrease the number of unique label values to reduce cardinality</li>
      <li>Drop the label entirely via&nbsp;
        <Hyperlink href='https://docs.victoriametrics.com/relabeling.html'>relabeling</Hyperlink></li>
      <li>For volatile label values (such as URL path, user session, etc.)
        consider printing them to the log file instead of adding to time series
      </li>
    </ul>
  </TipCard>
);

export const TipCardinalityOfSingle: FC = () => (
  <TipCard title="Dashboard of a single metric">
    <p>This dashboard helps to understand the cardinality of a single metric.</p>
    <p>
      Each time series is a unique combination of key-value label pairs.
      Therefore a label key with many values can create a lot of time series for a particular metric.
      If you’re trying to decrease the cardinality of a metric,
      start by looking at the labels with the highest number of values.
    </p>
    <p>Use the series selector at the top of the page to apply additional filters.</p>
  </TipCard>
);

export const TipCardinalityOfLabel: FC = () => (
  <TipCard title="Dashboard of a label">
    <p>
      This dashboard helps you understand the count of time series per label.
    </p>
    <p>
      Use the selector at the top of the page to pick a label name you’d like to inspect.
      For the selected label name, you’ll see the label values that have the highest number of series associated with
      them.
      So if you’ve chosen `instance` as your label name, you may see that `657` time series have value
      “host-1” attached to them and `580` time series have value `host-2` attached to them.
    </p>
    <p>
      This can be helpful in allowing you to determine where the bulk of your time series are coming from.
      If the label “instance=host-1” was applied to 657 series and the label “instance=host-2”
      was only applied to 580 series, you’d know, for example, that host-01 was responsible for sending
      the majority of the time series.
    </p>
  </TipCard>
);
