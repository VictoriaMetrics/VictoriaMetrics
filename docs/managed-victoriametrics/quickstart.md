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
# Quick Start in Cloud VictoriaMetrics


Cloud VictoriaMetrics - is a database-as-a-service platform, where users can run the VictoriaMetrics 
that they know and love on AWS without the need to perform typical DevOps tasks such as proper configuration, 
monitoring, logs collection, access protection, software updates, backups, etc.

The document covers the following topics
1. [How to register](#how-to-register)
1. [How to add payment method](#how-to-add-payment-method)
1. [How to restore password](#how-to-restore-password)
1. [Creating deployment](#creating-deployment)
1. [Deployment access](#deployment-access)
1. [Modifying deployment](#modifying-deployment)

## How to register

To register in the service it is easy to made a few steps:
1. Visit the main page of the [Cloud VictoriaMetrics](https://cloud.victoriametrics.com/signIn) and click into 
[Create an account link](https://cloud.victoriametrics.com/signUp).
<p>
   <img src="quick_start_signin.png" width="800">
</p>

There are two different methods to create an account:
1. Create an account via Google Auth service;
2. Create an account by filling all the form fields;

### Create an account via Google Auth service:

1. In the [signup page](https://cloud.victoriametrics.com/signUp) click into `Continue with Google` button

<p>
   <img src="quick_start_signup_google_click.png" width="800">
</p>

1. Choose email with which the account will be created

<p>
   <img src="quick_start_signup_choose_google_account.png" width="800">
</p>

1. If all successfully finished, the system will automatically redirect to the main page of the Cloud VictoriaMetrics.

<p>
   <img src="quick_start_signup_success.png" width="800">
</p>

### Create an account by filling form:
1. Fill all fields in [signup page](https://cloud.victoriametrics.com/signUp).

<p>
   <img src="quick_start_signup.png" width="800">
</p>

All fields are required and should be filled. All mistakes will be shown in the interface, 
so it is easy to understand what should be corrected.

<p>
   <img src="quick_start_signup_errors.png" width="800">
</p>

1. When all fields are filled correctly, the next step is to press `Create account` button

<p>
   <img src="quick_start_signup_create_account_click.png" width="800">
</p>

After correct signup process service will redirect to the main page with a notification message and email
like in the pictures below

1. Main page of the Cloud VictoriaMetrics
<p>
   <img src="quick_start_signup_success.png" width="800">
</p>

1. Confirmation email
<p>
   <img src="quick_start_signup_email_confirm.png" width="800">
</p>

It is necessary to confirm the email address. In other case, all actions in the service are not active.

After successfully email confirmation, it is easy to [create deployment](#creating-deployment) or [add payment method](#how-to-add-payment-method).
<p>
   <img src="quick_start_signup_email_confirmed.png" width="800">
</p>

## How to add payment method

To add a payment method it is necessary to:

1. Click into `Upgrade button` or `billing` menu item like in the picture below

<p>
  <img src="how_to_add_payment_method_upgrade.png" width="800">
</p>

2. Choose payment method

<p>
  <img src="how_to_add_payment_method_choose_method.png" width="800">
</p>

### Add subscription by payment card

1. Click into an `Add card` panel and fill all fields in the form and press `Add card` button

<p>
  <img src="how_to_add_payment_method_add_card.png" width="800">
</p>

2. If the card was invalid, the error message will appear

<p>
  <img src="how_to_add_payment_method_invalid_card.png" width="800">
</p>

3. If the card is added successfully, it will be shown in the interface

<p>
  <img src="how_to_add_payment_method_card_added.png" width="800">
</p>

### Subscribe via an AWS marketplace

If <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">AWS Marketplace</a> 
more preferable for subscription click into `AWS Card`

<p>
  <img src="how_to_add_payment_method_aws_click.png" width="800">
</p>

and service will be redirected to the <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">VictoriaMetrics product page</a>.
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

You'll be taken to <a href="https://cloud.victoriametrics.com/billing">Cloud VictoriaMetrics billing page</a>:

<p>
  <img src="how_to_add_payment_method_aws_finish.png" width="800">
</p>

### Change payment method

If both payment methods were enabled, it is possible to change the preferable payment method.
Click the radio button like in the picture below and confirm payment choice

<p>
  <img src="change_payment_method.png" width="800">
</p>

<p>
  <img src="change_payment_confirmation.png" width="800">
</p>

If the payment method changed, a success message will appear 

<p>
  <img src="change_payment_method_success.png" width="800">
</p>

## How to restore password

If you forgot password, it can be restored in the following way:

1. Click `Forgot password?` link at [this page](https://cloud.victoriametrics.com/signIn):

   <p>
     <img src="quick_start_restore_password.png" width="800">
   </p>

1. Enter your email in the field and click `Reset password` button:

   <p>
     <img src="quick_start_restore_password_email_field.png" width="800">
   </p>

   <p>
     <img src="quick_start_restore_password_message.png" width="800">
   </p>

1. Follow the instruction sent to your email in order to gain access to your VictoriaMetrics cloud account:

   <p>
     <img src="quick_start_restore_password_email.png" width="800">
   </p>

1. Navigate to the Profile page by clicking the corresponding link in the top right corner:

   <p>
     <img src="quick_start_restore_password_profile_click.png" width="800">
   </p>

1. Enter new password at the Profile page and press `Save` button:

   <p>
     <img src="quick_start_restore_password_profile_fields.png" width="800">
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
