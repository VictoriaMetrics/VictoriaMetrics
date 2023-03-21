import { TipIcon } from "../../../components/Main/Icons";
import React, { FC } from "preact/compat";
import { ReactNode } from "react";
import "./style.scss";

const Link: FC<{ href: string, children: ReactNode }> = ({ href, children }) => (
  <a
    href={href}
    className="vm-link vm-link_colored"
  >
    {children}
  </a>
);

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

export const TipDocumentation: FC = () => (
  <TipCard title="Cardinality explorer">
    <h6>Helpful for analyzing VictoriaMetrics TSDB data</h6>
    <ul>
      <li>
        <Link href="https://docs.victoriametrics.com/#cardinality-explorer">
          Cardinality explorer documentation
        </Link>
      </li>
      <li>
        See the <Link href="https://victoriametrics.com/blog/cardinality-explorer/">
        example of using</Link> the cardinality explorer
      </li>
    </ul>
  </TipCard>
);

export const TipHighNumberOfSeries: FC = () => (
  <TipCard title="Metrics with a high number of series">
    <ul>
      <li>
        You can to identify and eliminate labels with frequently changed values to reduce its&nbsp;
        <Link href='https://docs.victoriametrics.com/FAQ.html#what-is-high-cardinality'>cardinality</Link>&nbsp;and&nbsp;
        <Link href='https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate'>high churn rate</Link>
      </li>
      <li>
         You can find unused metrics and <Link href='https://docs.victoriametrics.com/relabeling.html'>drop entire metrics</Link>
      </li>
      <li>
          You can replace a large number of underlying series with <Link href='https://docs.victoriametrics.com/stream-aggregation.html'>streaming aggregation</Link>
      </li>
    </ul>
  </TipCard>
);

export const TipHighNumberOfValues: FC = () => (
  <TipCard title="Labels with a high number of unique values">
    <ul>
      <li>You can decrease its number of values</li>
      <li>You can drop this label entirely</li>
      <li>If you still this label with its values, could you store it in a log file?</li>
    </ul>
  </TipCard>
);

export const TipCardinalityOfSingle: FC = () => (
  <TipCard title="Dashboard of a single metric">
    <p>
      This dashboard helps you understand the cardinality of a single metric.
      It shows you the count of labels with this metric name and how that count relates
      to the total number of time series.
      Then it helps you understand which labels associated
      with that metric have the greatest impact on its cardinality.
    </p>
    <p>
      Each time series is a unique combination of key-value label pairs.
      Therefore a label key with a lot of values can create a lot of time series for a particular metric.
      If you’re trying to decrease the cardinality of a metric,
      start by looking at the labels with the highest number of values.
    </p>
    <p>
      Use the selector at the top of the page to pick which metric you’d like to inspect.
    </p>
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
