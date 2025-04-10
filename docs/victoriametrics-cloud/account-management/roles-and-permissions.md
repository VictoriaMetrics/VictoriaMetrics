---
weight: 3
title: Roles and Permissions
disableToc: true
menu:
  docs:
    parent: account-management
    weight: 3
---

## User roles

VictoriaMetrics Cloud provides different levels of user access based on role definitions.
Roles determine the information that users can access and edit inside VictoriaMetrics Cloud in
different `Categories`, such as Deployments, Billing or Notifications, for example. The full list of roles
definitions can be found in the [table](#roles-and-permissions) below.

Organization Administrators can assign and change other users roles both during the user creation procedure or afterwards. See the [User Management](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/user-management/)
section for more information.

### Roles and permissions

<table class="params">
  <tr>
   <td><strong>User Role</strong></td>
   <td><strong>Categories</strong></td>
   <td><strong>Permissions</strong></td>
  </tr>
  <tr>
   <td rowspan="7" ><strong>Admin</strong></td>
   <td>Deployments</td>
   <td>
    Access to all deployments tabs and information
    <p>Create, update and delete deployment</p>
   </td>
  </tr>
  <tr>
   <td>Integrations</td>
   <td>Access to different integration configurations</td>
  </tr>
  <tr>
   <td>Billing</td>
   <td>Check billing information</td>
  </tr>
  <tr>
   <td>Notifications</td>
   <td>Create and update notifications</td>
  </tr>
  <tr>
   <td>Audit Logs</td>
   <td>Can check all information in audit logs</td>
  </tr>
  <tr>
   <td>User Management</td>
   <td>Add, edit and  delete users</td>
  </tr>
  <tr>
   <td>API Keys</td>
   <td>Add, edit and  delete API Keys</td>
  </tr>
  <tr>
   <td rowspan="3"><strong>Editor</strong></td>
   <td>Deployments</td>
   <td>
    Access to all deployments tabs and information
    <p>Create, update and delete deployment</p>
   </td>
  </tr>
  <tr>
   <td>Notifications</td>
   <td>Create and update notifications</td>
  </tr>
  <tr>
   <td>Audit Logs</td>
   <td>Can check all information in audit logs</td>
  </tr>
  <tr>
   <td><strong>Viewer</strong></td>
   <td>Deployments</td>
   <td>Access to Overview, Monitoring, Explore and Alerts deployments tabs and information</td>
  </tr>
</table>

### Profile status

Profile lifecycle comprises different statuses depending on where they are in their registration process.
If you think your profile is in a wrong status or need assistance, don't hesitate to contact our
support team at support-cloud@victoriametrics.com.

<table class="params">
  <tr>
   <td><strong>Active</strong></td>
   <td>The user can log in and use VictoriaMetrics Cloud. The user role defines the access level.</td>
  </tr>
  <tr>
   <td><strong>Pending Invitation</strong></td>
   <td>An invitation was sent. The user must accept this.</td>
  </tr>
  <tr>
   <td><strong>Expired Invitation</strong></td>
   <td>An invitation was expired. The admin should resend invitation to the user.</td>
  </tr>
  <tr>
   <td><strong>Inactive</strong></td>
   <td>The user is registered in the VictoriaMetrics Cloud but has no access to perform any actions. Admin can activate or completely delete the user.</td>
  </tr>
</table>
