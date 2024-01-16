---
# sort: 2
weight: 2
title: Custom Model Guide
# disableToc: true
menu:
  docs:
    parent: "vmanomaly-models"
    weight: 2
    # sort: 2
aliases:
  - /anomaly-detection/components/models/custom_model.html
---

# Custom Model Guide
**Note**: vmanomaly is a part of [enterprise package](https://docs.victoriametrics.com/enterprise.html). Please [contact us](https://victoriametrics.com/contact-us/) to find out more.

Apart from vmanomaly predefined models, users can create their own custom models for anomaly detection.

Here in this guide, we will 
- Make a file containing our custom model definition
- Define VictoriaMetrics Anomaly Detection config file to use our custom model
- Run service

**Note**: The file containing the model should be written in [Python language](https://www.python.org/) (3.11+)

## 1. Custom model
We'll create `custom_model.py` file with `CustomModel` class that will inherit from vmanomaly `Model` base class.
In the `CustomModel` class there should be three required methods - `__init__`, `fit` and `infer`:
* `__init__` method should initiate parameters for the model.

  **Note**: if your model relies on configs that have `arg` [key-value pair argument](./models.md#section-overview), do not forget to use Python's `**kwargs` in method's signature and to explicitly call 
  ```python 
  super().__init__(**kwargs)
  ``` 
  to initialize the base class each model derives from
* `fit` method should contain the model training process.
* `infer` should return Pandas.DataFrame object with model's inferences.

For the sake of simplicity, the model in this example will return one of two values of `anomaly_score` - 0 or 1 depending on input parameter `percentage`.

<div class="with-copy" markdown="1">

```python
import numpy as np
import pandas as pd
import scipy.stats as st
import logging

from model.model import Model
logger = logging.getLogger(__name__)


class CustomModel(Model):
    """
    Custom model implementation.
    """

    def __init__(self, percentage: float = 0.95, **kwargs):
        super().__init__(**kwargs)
        self.percentage = percentage
        self._mean = np.nan
        self._std = np.nan

    def fit(self, df: pd.DataFrame):
        # Model fit process: 
        y = df['y']
        self._mean = np.mean(y)
        self._std = np.std(y)
        if self._std == 0.0:
            self._std = 1 / 65536


    def infer(self, df: pd.DataFrame) -> np.array:
        # Inference process:
        y = df['y']
        zscores = (y - self._mean) / self._std
        anomaly_score_cdf = st.norm.cdf(np.abs(zscores))
        df_pred = df[['timestamp', 'y']].copy()
        df_pred['anomaly_score'] = anomaly_score_cdf > self.percentage
        df_pred['anomaly_score'] = df_pred['anomaly_score'].astype('int32', errors='ignore')

        return df_pred

```

</div>


## 2. Configuration file
Next, we need to create `config.yaml` file with VM Anomaly Detection configuration and model input parameters.
In the config file `model` section we need to put our model class `model.custom.CustomModel` and all parameters used in `__init__` method.
You can find out more about configuration parameters in vmanomaly docs.

<div class="with-copy" markdown="1">

```yaml
scheduler:
  infer_every: "1m"
  fit_every: "1m"
  fit_window: "1d"

model:
  # note: every custom model should implement this exact path, specified in `class` field
  class: "model.model.CustomModel"
  # custom model params are defined here
  percentage: 0.9

reader:
  datasource_url: "http://localhost:8428/"
  queries:
    ingestion_rate: 'sum(rate(vm_rows_inserted_total)) by (type)'
    churn_rate: 'sum(rate(vm_new_timeseries_created_total[5m]))'

writer:
  datasource_url: "http://localhost:8428/"
  metric_format:
    __name__: "custom_$VAR"
    for: "$QUERY_KEY"
    model: "custom"
    run: "test-format"

monitoring:
  # /metrics server.
  pull:
    port: 8080
  push:
    url: "http://localhost:8428/"
    extra_labels:
      job: "vmanomaly-develop"
      config: "custom.yaml"
```

</div>

## 3. Running model
Let's pull the docker image for vmanomaly:

<div class="with-copy" markdown="1">

```sh 
docker pull us-docker.pkg.dev/victoriametrics-test/public/vmanomaly-trial:latest
```

</div>

Now we can run the docker container putting as volumes both config and model file:

**Note**: place the model file to `/model/custom.py` path when copying
<div class="with-copy" markdown="1">

```sh
docker run -it \
--net [YOUR_NETWORK] \
-v [YOUR_LICENSE_FILE_PATH]:/license.txt \
-v $(PWD)/custom_model.py:/vmanomaly/src/model/custom.py \
-v $(PWD)/custom.yaml:/config.yaml \
us-docker.pkg.dev/victoriametrics-test/public/vmanomaly-trial:latest /config.yaml \
--license-file=/license.txt
```
</div>

Please find more detailed instructions (license, etc.) [here](/vmanomaly.html#run-vmanomaly-docker-container)


## Output
As the result, this model will return metric with labels, configured previously in `config.yaml`.
In this particular example, 2 metrics will be produced. Also, there will be added other metrics from input query result.

```
{__name__="custom_anomaly_score", for="ingestion_rate", model="custom", run="test-format"}

{__name__="custom_anomaly_score", for="churn_rate", model="custom", run="test-format"}
```
