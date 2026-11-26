---
id: push-setup
title: Push setup
type: runbook
summary: Generate a VAPID key pair, turn Web Push on in local dev and production, and confirm delivery with Send test.
tags: [push, web-push, vapid, notifications, secret-manager, runbook]
---

# Push setup

## Purpose

Turn on Web Push. The app ships with push off, because it needs a VAPID key pair before it can subscribe a device or
send a notification. This runbook generates the pair, stores the private half, sets both variables on the service, and
checks delivery with Send test.

## Preconditions

- The app runs. For production, the Cloud Run service is deployed and `BASE_URL` is its public `https` URL, because the
  app uses that URL as the VAPID subject.
- `gcloud` is signed in with permission to update the Cloud Run service and add Secret Manager secrets. The project ID
  and region match the deploy variables.
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

### 5. Check delivery

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

## Rollback

Remove both variables from the service:

```sh
gcloud run services update parlay-club \
  --project "$GCP_PROJECT" --region "$GCP_REGION" \
  --remove-env-vars VAPID_PUBLIC_KEY,VAPID_PRIVATE_KEY
```

The Notifications card disappears, the push routes answer `404`, and nothing is sent. Stored subscriptions remain, and a
browser keeps its local subscription until someone turns notifications off. Delete the secrets if the pair is no longer
needed.

## Verify

- The Cloud Run revision has both variables and boots without a config error.
- `/me` shows the Notifications card, and opening it in a browser without a stored subscription says Off for this
  device.
- Send test delivers to a device that shows On, and the notification opens `/me`.
- A second person's devices receive only their own test, not the first person's.

## Troubleshooting

| Symptom                                      | Cause and fix                                                                                  |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| No Notifications card on `/me`               | One VAPID variable is missing or empty in the running revision. Set both and restart.           |
| Card says notifications are unsupported      | The page is not a secure context, or an iPhone is in Safari instead of the installed app.       |
| Card says Blocked for this device            | The browser's site settings deny notifications. Allow them, then reload the page.              |
| Send test reports success but nothing arrives | The device has no stored endpoint, focus mode is on, or OS notifications are off for the app.  |
| Logs show `push: send` with status 403        | The VAPID pair rotated. Turn notifications off and on again on that device.                      |
| Subscribe answers 400 with an unknown host    | The browser uses a push host off the allowlist. Add it to `internal/push` and redeploy.         |
