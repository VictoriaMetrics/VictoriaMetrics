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
# Quick Start

Managed VictoriaMetrics – is a database-as-a-service platform, where users can run the VictoriaMetrics 
that they know and love on AWS without the need to perform typical DevOps tasks such as proper configuration, 
monitoring, logs collection, access protection, software updates, backups, etc.

The document covers the following topics:
1. [Registration](#registration)
1. [Adding a payment method](#adding-a-payment-method)
1. [Restoring a password](#restoring-a-password)
1. [Creating deployments](#creating-deployments)
1. [Start writing and reading data](#start-writing-and-reading-data)
1. [Modifying an existing deployment](#modifying-an-existing-deployment)

## Registration

Start your registration process by visiting the [Sign Up](https://cloud.victoriametrics.com/signUp?utm_source=website&utm_campaign=docs_quickstart) page.

There are two different methods to create an account:
1. Create an account via Google Auth service;
1. Create an account by filling in a registration form.

### Create an account via Google Auth service:

1. Click `Continue with Google` button on the [Sign Up page](https://cloud.victoriametrics.com/signUp?utm_source=website&utm_campaign=docs_quickstart)
   <p>
      <img src="quick_start_signup_google_click.webp" width="800">
   </p>
1. Choose Google account you want to use for registration
   <p>
      <img src="quick_start_signup_choose_google_account.webp" width="800">
   </p>
1. You will be automatically redirected to the main page of the Managed VictoriaMetrics
   <p>
      <img src="quick_start_signup_success.webp" width="800">
   </p>

### Create an account by filling in a registration form:
1. Fill in your email, password and password confirmation on [Sign Up page](https://cloud.victoriametrics.com/signUp?utm_source=website&utm_campaign=docs_quickstart).
   <p>
      <img src="quick_start_signup.webp" width="800">
   </p>
1. All fields are required. Any errors will be shown in the interface, so it is easy to understand what should be adjusted.
   <p>
      <img src="quick_start_signup_errors.webp" width="800">
   </p>
1. Press `Create account` button when all fields are filled in.
   <p>
      <img src="quick_start_signup_create_account_click.webp" width="800">
   </p>

You will be redirected to the main page with a notification message to confirm your email.
<p>
   <img src="quick_start_signup_success.webp" width="800">
</p>

You will also receive an email with a confirmation link as shown on the picture below:
<p>
   <img src="quick_start_signup_email_confirm.webp" width="800">
</p>

It is necessary to confirm your email address. Otherwise, you won't be able to create a deployment.

After successful confirmation of your email address, you'll be able to [create your first deployment](#creating-deployments) or [add a payment method](#adding-a-payment-method).
<p>
   <img src="quick_start_signup_email_confirmed.webp" width="800">
</p>

## Adding a payment method

1. Navigate to a [Billing](https://cloud.victoriametrics.com/billing?utm_source=website&utm_campaign=docs_quickstart) page or click on `Upgrade` button as shown below:
   <p>
     <img src="how_to_add_payment_method_upgrade.webp" width="800">
   </p>

1. Choose a payment method
   <p>
     <img src="how_to_add_payment_method_choose_method.webp" width="800">
   </p>

### Pay with a card

1. Click on an `Add card` panel and fill in all the fields in the form and press `Add card` button
   <p>
     <img src="how_to_add_payment_method_add_card.webp" width="800">
   </p>
1. An error message will appear if a card us invalid
   <p>
     <img src="how_to_add_payment_method_invalid_card.webp" width="800">
   </p>
1. Successfully added card will be shown on the page as follows:
   <p>
     <img src="how_to_add_payment_method_card_added.webp" width="800">
   </p>

### Link your AWS billing account via AWS Marketplace

When you need to unify your AWS billing, you can start a subscription on AWS Marketplace.

1. Click on the `Buy on AWS Marketplace` panel:
   <p>
     <img src="how_to_add_payment_method_aws_click.webp" width="800">
   </p>
1. You will be redirected to the <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">Managed VictoriaMetrics</a> product page.
1. Click on `View purchase option` button, and you will be redirected to an AWS login page or to a subscribe page on AWS Marketplace.
   <p>
     <img src="quickstart_aws-purchase-click.webp" width="800">
   </p>
1. Go to the <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc">Managed VictoriaMetrics</a> product page and click `Continue to Subscribe` button:
   <p>
     <img src="quickstart_continue-subscribe.webp" width="800">
   </p>
1. Press the `Subscribe` button:
   <p>
     <img src="quickstart_subscribe.webp" width="800">
   </p>
1. After that you will see a success message where you should click `Set up your account` button:
   <p>
     <img src="quickstart_setup-your-account.webp" width="800">
   </p>
1. You'll be redirected back to Managed VictoriaMetrics <a href="https://cloud.victoriametrics.com/billing?utm_source=website&utm_campaign=docs_quickstart" target="_blank">billing page</a>:
   <p>
     <img src="how_to_add_payment_method_aws_finish.webp" width="800">
   </p>

### Switching between payment methods

If both payment methods are added, it is possible to easily switch between them.
Click on the radio button like on the picture below and confirm the change:

<p>
  <img src="change_payment_method.webp" width="800">
</p>

<p>
  <img src="change_payment_confirmation.webp" width="800">
</p>

If the payment method was changed successfully, the following message will appear: 

<p>
  <img src="change_payment_method_success.webp" width="800">
</p>

## Password restoration

If you forgot your password, it can be restored in the following way:

1. Click `Forgot password?` link on the [Sign In](https://cloud.victoriametrics.com/signIn?utm_source=website&utm_campaign=docs_quickstart) page:
   <p>
     <img src="quick_start_restore_password.webp" width="800">
   </p>

1. Enter your email and click `Reset password` button:
   <p>
     <img src="quick_start_restore_password_email_field.webp" width="800">
   </p>

   <p>
     <img src="quick_start_restore_password_message.webp" width="800">
   </p>

1. Follow the instructions sent to your email in order to get access to your Managed VictoriaMetrics account:
   <p>
     <img src="quick_start_restore_password_email.webp" width="800">
   </p>

1. Navigate to the Profile page by clicking the corresponding link in the top right corner:
   <p>
     <img src="quick_start_restore_password_profile_click.webp" width="800">
   </p>

1. Enter a new password on the Profile page and press `Save`:
   <p>
     <img src="quick_start_restore_password_profile_fields.webp" width="800">
   </p>

## Creating deployments

On the [Deployments](https://cloud.victoriametrics.com/deployments?utm_source=website&utm_campaign=docs_quickstart) page you 
will see a list of your existing deployments and will be able to manage them. 

To create a deployment click on the button `Create Deployment` button:

<p>
  <img src="create_deployment_start.webp" width="800">
</p>

On the opened screen, choose parameters of your new deployment:

* `Deployment type` 
  * Single - for affordable, performant single-node deployments;
  * Cluster - for highly available and multi-tenant deployments;
* `Region` – AWS region where deployment will run;
* Desired `storage capacity` for storing metrics (you always can expand disk size later);
* `Retention` period for stored metrics.
* `Size` of your deployment [based on your needs](https://docs.victoriametrics.com/guides/understand-your-setup-size.html)

<p>
  <img src="create_deployment_form.webp" width="800">
</p>

When all parameters are configured, click on the `Create` button, and deployment will be created.

Once created, deployment will remain for a short period of time in `PROVISIONING` status 
while the hardware spins-up, just wait for a couple of minutes and reload the page. 
You'll also be notified via email once your deployment is ready to use:

<p>
  <img src="create_deployment_created.webp" width="800">
</p>

<p>
  <img src="create_deployment_active_email.webp" width="800">
</p>

## Start writing and reading data

After transition from `PROVISIONING` to `RUNNING` state, Managed VictoriaMetrics deployment is fully operational 
and is ready to accept write and read requests. 

Click on deployment name and navigate to the Access tab to get the access token:

<p>
  <img src="deployment_access.webp" width="800">
</p>

Access tokens are used in token-based authentication to allow an application to access the VictoriaMetrics API. 
Supported token types are `Read-Only`, `Write-Only` and `Read-Write`. Click on token created by default 
to see usage examples:

<p>
  <img src="deployment_access_write_example.webp" width="800">
</p>

<p>
  <img src="deployment_access_read_example.webp" width="800">
</p>

Follow usage examples in order to configure access to VictoriaMetrics for your Prometheus, 
Grafana or any other software.

## Modifying an existing deployment

Remember, you can always add, remove or modify existing deployment by changing its size or any parameters on the 
deployment's page. 
It is important to know that downgrade for cluster is currently not available.

<p>
  <img src="modify_deployment.webp" width="800">
</p>
 
To discover additional configuration options click on `Advanced Settings` button, so you should see the following:

<p>
  <img src="modify_deployment_additional_settings.webp" width="800">
</p>

In that section, additional params can be set:

* [`Deduplication`](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#deduplication) defines interval when deployment leaves a single raw sample with the biggest timestamp per each discrete interval;
* `Maintenance Window` when deployment should start an upgrade process if needed;
* `Settings` allow to define different flags for the deployment:

   1. [cluster components flags](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#list-of-command-line-flags).
   2. [single version flags](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#list-of-command-line-flags).

Please note, such an update requires a deployment restart and may result in a short downtime for single-node deployments.
