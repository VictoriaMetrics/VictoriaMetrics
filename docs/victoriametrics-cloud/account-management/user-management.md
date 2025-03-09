---
weight: 4
title: User Management in VictoriaMetrics Cloud
menu:
  docs:
    parent: account-management
    weight: 4
    name: User Management
aliases:
  - /victoriametrics-cloud/user-managment/index.html
  - /victoriametrics-cloud/user-management/index.html
  - /managed-victoriametrics/user-management/index.html
---
The User Management system enables VictoriaMetrics Cloud Administrators to control user access and
onboard or offboard users to their Organization. It categorizes users according to their needs and role.

Administrators can manage users in the [User Management section](https://cloud.victoriametrics.com/users), which provides a
user list where actions can be applied:


|               **User Management field**           | **Description**                   |
|------------------------------------|-----------------------------------|
| **`Email`**       | Registration user email. |
| **`Status`**      | User profile [status](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/roles-and-permissions#profile-status). |
| **`User Role`**   | Admin, Editor or Viewer. See description [here](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/roles-and-permissions#roles-and-permissions). |
| **`Created At`**  | Date on which this user was created. |
| **`Last Active`** | User's last login date and time.    |
| **`Auth method`** | User's [authentication method](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/registration-and-trial/#authentication-methods).    |
| **`Actions`**  | Click here to manage the user. |

## Adding Users

Users can be added to VictoriaMetrics Cloud by sending an invitation. Invitations can be sent by
clicking on `Invite User` in the [User Management section](https://cloud.victoriametrics.com/users).

After filling out the form, click on the `Invite` button. 
The user will be saved, and an invitation email to the provided email address will be sent. As a confirmation, you will see the success message.

> The invitation link is only active for 24 hours.

The user will remain at the `Pending Invitation` [status](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/roles-and-permissions#profile-status)
until the invitation is accepted. At his point the user is all set and transitions to the `Active` status.

## Updating Users

Users can be activated, deactivated or modified, including their role, under the `Actions` menu and selecting `Manage`.

## Deleting Users

Users can also be deleted from an Organization. Simply navigate to the [User Management section](https://cloud.victoriametrics.com/users),
and select `Delete user` under the `Actions` menu.

## Resending invitations

If an invitation is expired, you can always to resend the invite to the user, by clicking on the `Resend invitation` button.