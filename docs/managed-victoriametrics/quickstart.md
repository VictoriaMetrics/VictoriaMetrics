---
sort: 2
weight: 2
title: Quick Start
menu:
  docs:
    parent: "managed"
    weight: 2
aliases:
- /managed-victoriametrics/quickstart.html
---
# Quick Start in Managed VictoriaMetrics


Managed VictoriaMetrics - is a database-as-a-service platform, where users can run the VictoriaMetrics 
that they know and love on AWS without the need to perform typical DevOps tasks such as proper configuration, 
monitoring, logs collection, access protection, software updates, backups, etc.

The document covers the following topics
1. [How to register](#how-to-register)
1. [How to restore password](#how-to-restore-password)
1. [Creating deployment](#creating-deployment)
1. [Deployment access](#deployment-access)
1. [Modifying deployment](#modifying-deployment)

## How to register

Managed VictoriaMetrics id distributed via <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">AWS Marketplace</a>.
Please note, that initial registering is only possible via link from <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">AWS Marketplace</a>.
To start using the service, one should have already registered AWS account 
and visit <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">VictoriaMetrics product page</a>.
On that page click on `View purchase option` and you will be redirected to login page or to subscribe page.

<p>
  <img src="quickstart_aws-purchase-click.png" width="800">
</p>

Then, go to the
<a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc">VictoriaMetrics product page</a>
and click `Continue to Subscribe` button:

<p>
  <img src="quickstart_continue-subscribe.png" width="800">
</p>

Then on product page press the `Subscribe` button:

<p>
  <img src="quickstart_subscribe.png" width="800">
</p>

After that action you will be able to see success message where you should click `Set up your account` button:

<p>
  <img src="quickstart_setup-your-account.png" width="800">
</p>

You'll be taken to <a href="https://dbaas.victoriametrics.com//signUp">Managed VictoriaMetrics sign up page</a>:

<p>
  <img src="quickstart_signup-page.png" width="800">
</p>

Choose to register manually or via Google Auth.

If it was chosen manually registration, confirmation email wil be sent to your email address.

<p>
  <img src="quickstart_email-confirm.png" width="800">
</p>

After Google Auth process will redirect automatically to the main page.

## How to restore password

If you forgot password, it can be restored in the following way:

1. Click `Forgot your password?` link at [this page](https://dbaas.victoriametrics.com/signIn):

   <p>
     <img src="quickstart_restore-password.png" width="800">
   </p>

1. Enter your email in the field and click `Send Email` button:

   <p>
     <img src="quickstart_restore-password-email.png" width="800">
   </p>

1. Follow the instruction sent to your email in order to gain access to your VictoriaMetrics cloud account:

   <p>
     <img src="quickstart_password-restore-email.png" width="800">
   </p>

1. Navigate to the Profile page by clicking the corresponding link at the top right corner:

   <p>
     <img src="quickstart_restore-password-profile.png" width="800">
   </p>

1. Enter new password at the Profile page and press `Save` button:

   <p>
     <img src="quickstart_restore-password-save-password.png" width="800">
   </p>

## Creating deployment

Deployments is a page where user can list and manage VictoriaMetrics deployments. 
To create a deployment click on the button `Create Deployment` button or link in the message:

<p>
  <img src="quickstart_deployments.png" width="800">
</p>

In the opened form, choose parameters of the new deployment such as:

* `Deployment type` from preset single or cluster deployments;
* `Region` where deployment should run;
* Desired `storage capacity` for storing metrics (you always can expand disk size later);
* `Retention` period for stored metrics.
* `Size` of your deployment

<p>
  <img src="quickstart_deployment-create.png" width="800">
</p>

When all parameters are entered, click on the `Create` button, and deployment will be created

Once created, deployment will remain for a short period of time in `PROVISIONING` status 
while the hardware spins-up, just wait for a couple of minutes and reload the page. 
You'll also be notified via email once provisioning is finished:

<p>
  <img src="quickstart_deployment-created.png" width="800">
</p>

<p>
  <img src="quickstart_deployments-running.png" width="800">
</p>

## Deployment access

After transition from `PROVISIONING` to `RUNNING` state, VictoriaMetrics is fully operational 
and ready to accept write or read requests. But first, click on deployment name to get the access token:

<p>
  <img src="quickstart_deployment-access-token.png" width="800">
</p>

Access tokens are used in token-based authentication to allow an application to access the VictoriaMetrics API. 
Supported token types are `Read-Only`, `Write-Only` and `Read-Write`. Click on token created by default 
to see usage examples:

<p>
  <img src="quickstart_read-token.png" width="800">
</p>

<p>
  <img src="quickstart_write-token.png" width="800">
</p>

Follow usage example in order to configure access to VictoriaMetrics for your Prometheus, 
Grafana or any other software.

## Modifying deployment

Remember, you always can add, remove or modify existing deployment by changing their size or any parameters on the 
update form.

<p>
  <img src="quickstart_update-deployment.png" width="800">
</p>

There is another options present to customise you deployment setup. 
To discover them click on `Customise` button

<p>
  <img src="quickstart_customise-deployment.png" width="800">
</p>

In that section additional params can be set:

* `Deduplication` defines interval when deployment leaves a single raw sample with the biggest timestamp per each discrete interval;
* `Maintenance Window` when deployment should start upgrade process if needed;
* `Settings` allow to define different flags for the deployment.

However, such an update requires a deployment restart and may result into a couple of minutes of downtime.
