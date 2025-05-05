---
title : "CloudWatch - Agentless AWS monitoring"
menu:
  docs:
    parent: "integrations"
---

VictoriaMetrics Cloud supports **agentless AWS monitoring** by integrating directly with
[Amazon CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/WhatIsCloudWatch.html)
via [Amazon Kinesis Data Firehose](https://docs.aws.amazon.com/firehose/latest/dev/what-is-this-service.html).
This allows you to forward metrics from AWS services (like EC2, RDS, Lambda, etc.) to VictoriaMetrics
Cloud without deploying any collectors or agents.

This integration provides a simple, scalable, and maintenance-free way to monitor your AWS infrastructure.

## Integrating CloudWatch via AWS Firehose

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access**, as metrics are pushed from AWS), follow the
steps in the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To set up agentless AWS monitoring using Firehose, visit the
[cloud console](https://console.victoriametrics.cloud/integrations/cloudwatch),
or follow this interactive guide:


<iframe 
    width="100%"
    style="aspect-ratio: 1/9;"
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/cloudwatch" >
</iframe>
