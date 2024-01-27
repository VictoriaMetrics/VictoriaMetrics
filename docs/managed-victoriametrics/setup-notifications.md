---
sort: 6
weight: 6
title: Notifications in Managed VictoriaMetrics
menu:
  docs:
    parent: "managed"
    weight: 6
aliases:
- /managed-victoriametrics/setup-notifications.html
---
# Notifications in Managed VictoriaMetrics

The guide covers how to enable email and Slack notifications.

Table of content:
1. [Setup Slack notifications](#setup-slack-notifications)
1. [Setup emails notifications](#setup-emails-notifications)
1. [Send test notification](#send-test-notification)

When you enter the notification section, you will be able to fill in the channels in which you
want to receive notifications

<img src="notifications_view.webp" >

## Setup Slack notifications

1. Setup Slack webhook
   How to do this is indicated on the following link 
<a href="https://api.slack.com/messaging/webhooks" target="_blank">https://api.slack.com/messaging/webhooks</a>

   <img src="notifications_view.webp" >

1. Specify Slack channels

   Enter one or more channels into input and press enter or choose it after each input.

     <img src="notifications_setup_slack.webp" >
     <img src="notifications_setup_slack_enter_channel.webp">

## Setup emails notifications

You can specify one or multiple emails for notifications in the input field. By default, 
email notifications are enabled for the account owner

  <img src="notifications_setup_emails.webp" >
  <img src="notifications_setup_emails_input.webp" >


## Send test notification

To test your notification settings, press Save and Test.

If only Slack channels and webhook are specified correctly, you will receive the notification in the Slack channels.
If only the emails are specified, you will receive notifications to those emails.
When both notifications are specified, all notifications will be sent to Slack channels and emails.

  <img src="notifications_save_and_test.webp" >

If the Save button is pressed, then entered channels will be only saved, and you get a success message.

If the Save and Test button is pressed, then all entered information will be saved, 
and test notifications will be sent to the entered channels

  <img src="notifications_save_success.webp" >

Examples of the test notification messages:

  <img src="notifications_slack_test.webp" >

  <img src="notifications_email_test.webp" >

