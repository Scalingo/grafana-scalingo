---
aliases:
  - ../contact-points/create-contact-point/
  - ../contact-points/delete-contact-point/
  - ../contact-points/edit-contact-point/
  - ../contact-points/message-templating/
  - ../contact-points/test-contact-point/
  - ../message-templating/
  - ../unified-alerting/message-templating/
keywords:
  - grafana
  - alerting
  - guide
  - contact point
  - templating
title: Manage contact points
weight: 100
---

# Manage contact points

Use contact points to define how your contacts are notified when an alert rule fires. You can create, edit, delete, and test a contact point.

## Add a contact point

Complete the following steps to add a contact point.

1. In the Grafana menu, click the **Alerting** (bell) icon to open the Alerting page listing existing alerts.
1. Click **Contact points** to open the page listing existing contact points.
1. Click **New contact point**.
1. From the **Alertmanager** dropdown, select an Alertmanager. By default, Grafana Alertmanager is selected.
1. In **Name**, enter a descriptive name for the contact point.
1. From **Contact point type**, select a type and fill out mandatory fields. For example, if you choose email, enter the email addresses. Or if you choose Slack, enter the Slack channel(s) and users who should be contacted.
1. Some contact point types, like email or webhook, have optional settings. In **Optional settings**, specify additional settings for the selected contact point type.
1. In Notification settings, optionally select **Disable resolved message** if you do not want to be notified when an alert resolves.
1. To add another contact point type, click **New contact point type** and repeat steps 6 through 8.
1. Click **Save contact point** to save your changes.

## Edit a contact point

Complete the following steps to edit a contact point.

1. In the Alerting page, click **Contact points** to open the page listing existing contact points.
1. Find the contact point to edit, then click **Edit** (pen icon).
1. Make any changes and click **Save contact point**.

## Delete a contact point

Complete the following steps to delete a contact point.

1. In the Alerting page, click **Contact points** to open the page listing existing contact points.
1. Find the contact point to delete, then click **Delete** (trash icon).
1. In the confirmation dialog, click **Yes, delete**.

> **Note:** You cannot delete contact points that are in use by a notification policy. You will have to either delete the notification policy or update it to use another contact point.

## Test a contact point

Complete the following steps to test a contact point.

1. In the Grafana side bar, hover your cursor over the **Alerting** (bell) icon and then click **Contact** points.
1. Find the contact point to test, then click **Edit** (pen icon). You can also create a new contact point if needed.
1. Click **Test** (paper airplane icon) to open the contact point testing modal.
1. Choose whether to send a predefined test notification or choose custom to add your own custom annotations and labels to include in the notification.
1. Click **Send test notification** to fire the alert.
