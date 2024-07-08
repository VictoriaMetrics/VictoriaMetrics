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

Managed VictoriaMetrics is a hosted monitoring platform, where users can run the VictoriaMetrics 
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
<img src="quick_start_signup_google_click.webp" >
   
1. Choose Google account you want to use for registration
<img src="quick_start_signup_choose_google_account.webp" >

1. You will be automatically redirected to the main page of the Managed VictoriaMetrics
<img src="quick_start_signup_success.webp" >

### Create an account by filling in a registration form:
1. Fill in your email, password and password confirmation on [Sign Up page](https://cloud.victoriametrics.com/signUp?utm_source=website&utm_campaign=docs_quickstart).
<img src="quick_start_signup.webp" >

1.All fields are required. Any errors will be shown in the interface, so it is easy to understand what should be adjusted.
   <img src="quick_start_signup_errors.webp" >

1. Press `Create account` button when all fields are filled in.
   <img src="quick_start_signup_create_account_click.webp" >

You will be redirected to the main page with a notification message to confirm your email.

   <img src="quick_start_signup_success.webp" >


You will also receive an email with a confirmation link as shown on the picture below:

   <img src="quick_start_signup_email_confirm.webp" >


It is necessary to confirm your email address. Otherwise, you won't be able to create a deployment.

After successful confirmation of your email address, you'll be able to [create your first deployment](#creating-deployments) or [add a payment method](#adding-a-payment-method).

   <img src="quick_start_signup_email_confirmed.webp" >


## Adding a payment method

1. Navigate to a [Billing](https://cloud.victoriametrics.com/billing?utm_source=website&utm_campaign=docs_quickstart) page or click on `Upgrade` button as shown below:
   
     <img src="how_to_add_payment_method_upgrade.webp" >
   

1. Choose a payment method
   
     <img src="how_to_add_payment_method_choose_method.webp" >
   

### Pay with a card

1. Click on an `Add card` panel and fill in all the fields in the form and press `Add card` button
   
     <img src="how_to_add_payment_method_add_card.webp" >
   
1. An error message will appear if a card us invalid
   
     <img src="how_to_add_payment_method_invalid_card.webp" >
   
1. Successfully added card will be shown on the page as follows:
   
     <img src="how_to_add_payment_method_card_added.webp" >
   

### Link your AWS billing account via AWS Marketplace

When you need to unify your AWS billing, you can start a subscription on AWS Marketplace.

1. Click on the `Buy on AWS Marketplace` panel:
   
     <img src="how_to_add_payment_method_aws_click.webp" >
   
1. You will be redirected to the <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc" target="_blank">Managed VictoriaMetrics</a> product page.
1. Click on `View purchase option` button, and you will be redirected to an AWS login page or to a subscribe page on AWS Marketplace.
   
     <img src="quickstart_aws-purchase-click.webp" >
   
1. Go to the <a href="https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc">Managed VictoriaMetrics</a> product page and click `Continue to Subscribe` button:
   
     <img src="quickstart_continue-subscribe.webp" >
   
1. Press the `Subscribe` button:
   
     <img src="quickstart_subscribe.webp" >
   
1. After that you will see a success message where you should click `Set up your account` button:
   
     <img src="quickstart_setup-your-account.webp" >
   
1. You'll be redirected back to Managed VictoriaMetrics <a href="https://cloud.victoriametrics.com/billing?utm_source=website&utm_campaign=docs_quickstart" target="_blank">billing page</a>:
   
     <img src="how_to_add_payment_method_aws_finish.webp" >
   

### Switching between payment methods

If both payment methods are added, it is possible to easily switch between them.
Click on the radio button like on the picture below and confirm the change:


  <img src="change_payment_method.webp" >



  <img src="change_payment_confirmation.webp" >


If the payment method was changed successfully, the following message will appear: 


  <img src="change_payment_method_success.webp" >


## Password restoration

If you forgot your password, it can be restored in the following way:

1. Click `Forgot password?` link on the [Sign In](https://cloud.victoriametrics.com/signIn?utm_source=website&utm_campaign=docs_quickstart) page:
   
     <img src="quick_start_restore_password.webp" >
   

1. Enter your email and click `Reset password` button:
   
     <img src="quick_start_restore_password_email_field.webp" >
   

   
     <img src="quick_start_restore_password_message.webp" >
   

1. Follow the instructions sent to your email in order to get access to your Managed VictoriaMetrics account:
   
     <img src="quick_start_restore_password_email.webp" >
   

1. Navigate to the Profile page by clicking the corresponding link in the top right corner:
   
     <img src="quick_start_restore_password_profile_click.webp" >
   

1. Enter a new password on the Profile page and press `Save`:
   
     <img src="quick_start_restore_password_profile_fields.webp" >
   

## Creating deployments

On the [Deployments](https://cloud.victoriametrics.com/deployments?utm_source=website&utm_campaign=docs_quickstart) page you 
will see a list of your existing deployments and will be able to manage them. 

To create a deployment click on the button `Start sending metrics` button:


  <img src="create_deployment_start.webp" >

When you already have at least one deployment you can create a new one by clicking on the `Create deployment` button:

   <img src="create_deployment_continue.webp">


On the opened screen, choose parameters of your new deployment:

* `Deployment type` 
  * Single - for affordable, performant single-node deployments;
  * Cluster - for highly available and multi-tenant deployments;
* `Region` – AWS region where deployment will run;
* Desired `storage capacity` for storing metrics (you always can expand disk size later);
* `Retention` period for stored metrics.
* `Size` of your deployment [based on your needs](https://docs.victoriametrics.com/guides/understand-your-setup-size.html)


  <img src="create_deployment_form.webp" >


When all parameters are configured, click on the `Create` button, and deployment will be created.

Once created, deployment will remain for a short period of time in `PROVISIONING` status 
while the hardware spins-up, just wait for a couple of minutes and reload the page. 
You'll also be notified via email once your deployment is ready to use:


  <img src="create_deployment_created.webp" >



  <img src="create_deployment_active_email.webp" >


## Start writing and reading data

After transition from `PROVISIONING` to `RUNNING` state, Managed VictoriaMetrics deployment is fully operational 
and is ready to accept write and read requests. 

Click on deployment name and navigate to the Access tab to get the access token:


  <img src="deployment_access.webp" >


Access tokens are used in token-based authentication to allow an application to access the VictoriaMetrics API. 
Supported token types are `Read-Only`, `Write-Only` and `Read-Write`. Click on token created by default 
to see usage examples:


  <img src="deployment_access_write_example.webp" >



  <img src="deployment_access_read_example.webp" >


Follow usage examples in order to configure access to VictoriaMetrics for your Prometheus, 
Grafana or any other software.

## Modifying an existing deployment

Remember, you can always add, remove or modify existing deployment by changing its size or any parameters on the 
deployment's page. 
It is important to know that downgrade for cluster is currently not available.


  <img src="modify_deployment.webp" >

 
To discover additional configuration options click on `Advanced Settings` button, so you should see the following:


  <img src="modify_deployment_additional_settings.webp" >


In that section, additional params can be set:

* [`Deduplication`](https://docs.victoriametrics.com/cluster-victoriametrics/#deduplication) defines interval when deployment leaves a single raw sample with the biggest timestamp per each discrete interval;
* `Maintenance Window` when deployment should start an upgrade process if needed;
* `Settings` allow to define different flags for the deployment:

   1. [cluster components flags](https://docs.victoriametrics.com/cluster-victoriametrics/#list-of-command-line-flags).
   2. [single version flags](https://docs.victoriametrics.com/single-server-victoriametrics/#list-of-command-line-flags).

Please note, such an update requires a deployment restart and may result in a short downtime for single-node deployments.
