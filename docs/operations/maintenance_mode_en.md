# Maintenance Mode

This document explains the maintenance mode and how to control it for the Cloudogu EcoSystem MultiNode.

Maintenance mode is a system state of the Ecosystem where external access to the EcoSystem is disabled. The mode is
required when system critical processes are running. While maintenance mode is activated, a maintenance page is
returned for each access to a Dogus.

# Activate Maintenance Mode

To put the CES into maintenance mode, the following string must be written to `/config/_global/maintenance`:

```json
{
  "title": "Dies ist der Titel",
  "text": "Das ist der Text"
}
``` 

Each request to the CES is then answered with the HTTP code 503 (Service Unavailable) until the key in the etcd is
either deleted. Thereby the content of `title` and `text` is displayed on the page.

**Note:** Enabling and disabling maintenance mode will cause the Nginx static dogus to restart. However, this should
only take a few seconds.

## Caution

Since the maintenance page is served by nginx, it is not possible to view the maintenance mode page while an upgrade of
Nginx is in progress.