---
id: push-setup
title: Push setup
type: runbook
summary: Generate a VAPID key pair, turn Web Push on in local dev and production, set up the Cloud Tasks reminder queue, and confirm delivery with Send test and the first-week check.
tags: [push, web-push, vapid, notifications, reminders, cloud-tasks, secret-manager, runbook]
budget:
  max_lines: 240
  reason: Web Push and kickoff reminders share one set of secrets, one service update, and one first-week delivery check.
---

# Push setup

## Purpose

Turn on Web Push and kickoff reminders. The app ships with push off, because it needs a VAPID key pair before it can
subscribe a device or send a notification. This runbook generates the pair, stores the private half, sets both
variables on the service, wires the Cloud Tasks reminder queue and its bearer token, and checks delivery with Send test
and the first-week check.

## Preconditions

- The app runs. For production, the Cloud Run service is deployed and `BASE_URL` is its public `https` URL, because the
  app uses that URL as the VAPID subject.
- `gcloud` is signed in with permission to update the Cloud Run service, add Secret Manager secrets, and read the Cloud
  Tasks queue. The project ID and region match the deploy variables.
- `deploy/setup.sh` has run, so the `cloudtasks.googleapis.com` API is enabled, the queue exists, and the runtime
  service account holds `roles/cloudtasks.enqueuer` on it.
- You can sign in to `/me` from the device you want to test. An iPhone needs iOS 16.4 or later and the installed app.
- The browser supports service workers and Web Push. Chrome, Edge, Firefox, and Safari in an installed app do.

## Steps

### 1. Generate a key pair

From the repo root:

```sh
go run ./cmd/vapidkeys
```

It prints a shell-sourceable pair:

```sh
VAPID_PUBLIC_KEY=...
VAPID_PRIVATE_KEY=...
```

Generate one pair and keep it. Copy the two assignments into your shell so the production steps can read them. The
public key identifies the server to the browser's push service, and the private key signs each send. The pair is not
interchangeable with a pair from another machine.

### 2. Turn it on locally

Add the pair from step 1 to the repo's env file, which `mise` loads. If you no longer have the output, generate a fresh
pair and append it:

```sh
go run ./cmd/vapidkeys >> .env
```

Restart `mise run dev` so the server reads the new variables. Open `http://localhost:8080/me` and sign in. The
Notifications card appears. Tap Turn on, allow the browser prompt, and the state changes to On for this device. Tap Send
test; the notification says "Notifications are on for this device."

`localhost` counts as a secure context, so local dev needs no TLS. A phone on the LAN does not: unless it reaches the
dev server over `https` or through a tunnel, the card reports that notifications are unsupported.

### 3. Store the keys for production

Keep the pair in Secret Manager so a rotation has one source and the private half never sits in the repository. Create
both secrets once:

```sh
printf '%s' "$VAPID_PUBLIC_KEY" | gcloud secrets create vapid-public-key \
  --replication-policy=automatic --data-file=-
printf '%s' "$VAPID_PRIVATE_KEY" | gcloud secrets create vapid-private-key \
  --replication-policy=automatic --data-file=-
```

If they already exist, add versions instead:

```sh
printf '%s' "$VAPID_PUBLIC_KEY" | gcloud secrets versions add vapid-public-key --data-file=-
printf '%s' "$VAPID_PRIVATE_KEY" | gcloud secrets versions add vapid-private-key --data-file=-
```

The public key is not sensitive, but keeping it beside its private half avoids a service that has one and not the other.

### 4. Set the service environment

The server reads both values from the environment. Read the pair from Secret Manager and update the service in one call:

```sh
VAPID_PUBLIC_KEY=$(gcloud secrets versions access latest --secret=vapid-public-key)
VAPID_PRIVATE_KEY=$(gcloud secrets versions access latest --secret=vapid-private-key)
gcloud run services update parlay-club \
  --project "$GCP_PROJECT" --region "$GCP_REGION" \
  --update-env-vars "VAPID_PUBLIC_KEY=$VAPID_PUBLIC_KEY,VAPID_PRIVATE_KEY=$VAPID_PRIVATE_KEY"
```

`--update-env-vars` changes only these two variables and leaves the rest of the service environment alone. Later deploys
also update env vars, so the pair survives. The deploy workflow does not mount the secret itself, so this call is what
copies the pair from Secret Manager into the revision's environment.

### 5. Set up kickoff reminders

Reminders need the queue that `deploy/setup.sh` created, a bearer token, and both variables on the service. Confirm the
queue and read its full path:

```sh
gcloud tasks queues describe parlay-club-reminders \
  --project "$GCP_PROJECT" --location "$GCP_REGION" \
  --format='value(name)'
```

The path has the shape `projects/{project}/locations/{region}/queues/parlay-club-reminders`. The queue allows three
attempts with a 30 second minimum backoff. The runtime account's `roles/cloudtasks.enqueuer` grant on this queue is what
lets the app enqueue, so a `403` here means the grant is missing.

Create the token secret once, then add versions when it changes:

```sh
TOKEN=$(openssl rand -hex 32)
printf '%s' "$TOKEN" | gcloud secrets create reminder-token \
  --replication-policy=automatic --data-file=-
```

If the secret already exists:

```sh
printf '%s' "$(openssl rand -hex 32)" | gcloud secrets versions add reminder-token --data-file=-
```

Set both values on the service:

```sh
REMINDER_TOKEN=$(gcloud secrets versions access latest --secret=reminder-token)
QUEUE_PATH=$(gcloud tasks queues describe parlay-club-reminders \
  --project "$GCP_PROJECT" --location "$GCP_REGION" --format='value(name)')
gcloud run services update parlay-club \
  --project "$GCP_PROJECT" --region "$GCP_REGION" \
  --update-env-vars "REMINDER_TOKEN=$REMINDER_TOKEN,REMINDER_QUEUE=$QUEUE_PATH"
```

`--update-env-vars` leaves the VAPID pair and the rest of the environment alone, and later deploys update only their own
variables, so both reminder values survive. The deploy workflow does not mount `reminder-token`, so this call is what
puts it in the revision. Rotation is the same as adding a version and repeating the update.

Reminders also stay off while push is off, because there is no sender. Do this step after the VAPID pair is set. A bad
queue path or `BASE_URL` logs one `reminders: cloud tasks scheduler; reminders are off` error at boot; it does not stop
the service.

### 6. Check delivery

1. Open `/me` in the installed app or a desktop browser and confirm the Notifications card is present.
2. Tap Turn on and allow the prompt. Confirm the card says On for this device and shows Turn off and Send test.
3. Tap Send test and confirm the notification arrives. Tapping it opens `/me`.
4. Ask one other person to turn notifications on and press Send test. Only their devices should buzz.

## Rotating the pair

Generate a new pair, add both values as new secret versions, and repeat step 4. Every browser subscription was created
with the old public key, so sends signed with the new key fail with a `403` and those rows stay stored. Only a `404` or
`410` prunes an endpoint.

Announce the rotation, then have each person turn notifications off and on, which replaces the subscription with one
signed for the new key. There is no automatic cleanup for a rotation.

## First-week reminder check

Run this during the first game week after reminders go on. It catches a missing queue grant, a bad queue path, and a
scheduler that never runs before anyone has to report a silent phone.

1. List the queue's pending tasks. Each open slip should have one task, named with a short hash and its slip id, due 20
   minutes before that slip's first kickoff:

   ```sh
   gcloud tasks list --queue=parlay-club-reminders \
     --project "$GCP_PROJECT" --location "$GCP_REGION"
   ```

2. Open the app once from a signed-in account before kickoff. That page load runs `reminders.Sync`, which creates the
   tasks. A slip made by hand schedules its task on the create request.
3. Check the logs for `reminders: scheduled` with the slip, start, and fire time. A `reminders: enqueue` warning means
   the task was not created; read the error and fix the queue path, the token, or the IAM grant.
4. Confirm the reminder arrives 15 to 30 minutes before the first kickoff. Tapping it opens `/slips/{id}`.
5. Confirm only people who still need legs or have not locked in got one, and that nobody gets a second reminder before
   a later kickoff wave that week.

A task fires but sends nothing is normal when everyone locked in, the slip graded, or the kickoff moved. The response
says which: `{skipped: "stale"}` when the start or status changed, `{skipped: "nobody"}` when every member is done.

## Rollback

Remove both variables from the service:

```sh
gcloud run services update parlay-club \
  --project "$GCP_PROJECT" --region "$GCP_REGION" \
  --remove-env-vars VAPID_PUBLIC_KEY,VAPID_PRIVATE_KEY
```

The Notifications card disappears, the push routes answer `404`, and nothing is sent. Reminders also stop, because there
is no sender, even though their endpoint and tasks still exist. Stored subscriptions remain, and a browser keeps its
local subscription until someone turns notifications off. Delete the secrets if the pair is no longer needed.

To remove reminders as well, drop their two variables and optionally the queue:

```sh
gcloud run services update parlay-club \
  --project "$GCP_PROJECT" --region "$GCP_REGION" \
  --remove-env-vars REMINDER_TOKEN,REMINDER_QUEUE
```

## Verify

- The Cloud Run revision has both VAPID variables and boots without a config error.
- `/me` shows the Notifications card, and opening it in a browser without a stored subscription says Off for this
  device.
- Send test delivers to a device that shows On, and the notification opens `/me`.
- A second person's devices receive only their own test, not the first person's.
- The service has `REMINDER_TOKEN` and `REMINDER_QUEUE`, and the boot log has no `reminders: cloud tasks scheduler;
  reminders are off` error.
- The queue holds one task per open slip, due 20 minutes before its first kickoff, and the first week's reminder arrives
  in the 15–30 minute window with no second one before later kickoffs.

## Troubleshooting

| Symptom                                      | Cause and fix                                                                                  |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| No Notifications card on `/me`               | One VAPID variable is missing or empty in the running revision. Set both and restart.           |
| Card says notifications are unsupported      | The page is not a secure context, or an iPhone is in Safari instead of the installed app.       |
| Card says Blocked for this device            | The browser's site settings deny notifications. Allow them, then reload the page.              |
| Send test reports success but nothing arrives | The device has no stored endpoint, focus mode is on, or OS notifications are off for the app.  |
| Logs show `push: send` with status 403        | The VAPID pair rotated. Turn notifications off and on again on that device.                      |
| Subscribe answers 400 with an unknown host    | The browser uses a push host off the allowlist. Add it to `internal/push` and redeploy.         |
| Queue shows no reminder tasks                 | `REMINDER_QUEUE` or `REMINDER_TOKEN` is missing, or no page load ran `Sync`. Open the app and check the logs. |
| Logs show `reminders: enqueue` status 403     | The runtime service account lacks `roles/cloudtasks.enqueuer` on the queue. Rerun `deploy/setup.sh`. |
| Logs show `reminders: cloud tasks scheduler; reminders are off` | `REMINDER_QUEUE` is not a full `projects/.../queues/...` path, or `BASE_URL` is not an `http`/`https` URL. Fix and restart. |
| Tasks exist but no reminder arrives           | Push is off, the device is unsubscribed, or everyone had already locked in. Check the fire's `{skipped: ...}` response. |
