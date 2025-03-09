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
The user management system enables admins to control user access and onboard and offboard users to the VictoriaMetrics Cloud. It organizes users according to their needs and role.

The document covers the following topics
- [User list](#user-list)
- [How to Add User](#how-to-add-user)
- [How to Update User](#how-to-update-user)
- [How to Delete User](#how-to-delete-user)
- [How to resend invitation](#how-to-resend-invitation)



## User list

It shows all users with different roles, and you can apply provided actions with them.

![User Management list](user_management_list.webp)

In the table, there is additional information about the users:


<table class="params">
  <tr>
   <td>Email:</td>
   <td>Registration user email</td>
  </tr>
  <tr>
   <td>Status:</td>
   <td>Active, Inactive or Pending Invitation</td>
  </tr>
  <tr>
   <td>User Role:</td>
   <td>Admin, Editor or Viewer</td>
  </tr>
  <tr>
   <td>Created At:</td>
   <td>When user was created</td>
  </tr>
  <tr>
   <td>Last Active:</td>
   <td>Last login date and time</td>
  </tr>
  <tr>
   <td>Auth method:</td>
   <td>Auth options to login into the VictoriaMetrics Cloud</td>
  </tr>
  <tr>
   <td>Actions:</td>
   <td>Actions to manipulate with users</td>
  </tr>
</table>

## How to Add User

Click on `Invite user` button 
the user invitation button and fill out the form in the modal, which will appear after you click. All fields are mandatory.

![Invite user](user_management_invite_user.webp)

After filling out the form, click on the `Invite` button. 
The user will be saved, and an invitation email to the provided email address will be sent. As a confirmation, you will see the success message.

**The invitation link is only active for 24 hours.**

The user will be at Pending Invitation status. After accepting the invitation user status changes to Active.


![Invite success](user_management_invite_success.webp)


## How to Update User

To edit the user role and details, activate or deactivate a user, and click on


![Update user](user_management_update_user.webp)

![Update user dropdown](user_management_update_user_dropdown.webp)

User editing form:

![Update form](user_management_user_update_form.webp)

To save changes, click the `Update` button. If changes are saved successfully, you will see a message at the top of the page.

![Updated success](user_management_user_updated_success.webp)

## How to Delete User

You can delete a user from your account. However, you will need to invite them again if you want to give them access.

Click on `Delete` button to delete the user.

![User delete](user_management_user_delete.webp)

To confirm the deletion of a user, you will need to re-enter their email address and press the **Confirm** button

![User delete form](user_management_delete_user_form.webp)

If the user is deleted successfully, you will see a success message

![Delete success](user_management_delete_success.webp)

## How to resend invitation

If the invitation is expired, it is possible to resend email to the user

Click `Resend invitation` button

![Resend invitation](user_management_resend_invitation.webp)

Confirm resend invitation by clicking `Confirm` button in the modal dialog

![Confirm resend invitation](user_management_confirm_resend_invitation.webp)

If invitation successfully resented to the user success message will appear

![Resend success](user_management_resend_success.webp)
