---
id: google-oauth-setup
title: Google OAuth setup
type: runbook
summary: Create the Google Web client, register the local and prod redirect URIs, publish the consent screen, and set the client secrets.
tags: [google, oauth, setup, secrets, deploy, runbook]
sources: [internal/auth/provider.go, internal/auth/handlers.go, internal/config/config.go, cmd/server/main.go]
last_verified: 2026-11-05
---

# Google OAuth setup

## Purpose

Put Google sign-in to work for local development and production. This runbook creates one OAuth Web client, registers
the two callback URLs, publishes the consent screen, and gives the app its client ID, client secret, and allow list.

The app uses the authorization code flow with PKCE. It asks only for `openid email profile`, stores the verified
identity, and calls no other Google API.

## Preconditions

- Access to the Google Cloud project that should own the OAuth client, with permission to edit OAuth clients and
  Secret Manager.
- The local app runs at `http://localhost:8080` unless you set another `PORT` and `BASE_URL`.
- For prod, the Cloud Run service URL or custom domain, for example `https://parlay-club-abc123.us-central1.run.app`.
- Secret Manager secrets for `session-key`, `google-client-id`, `google-client-secret`, and `odds-api-key`. The deploy
  setup creates them empty; you add versions here.

## Steps

### 1. Configure the consent screen

1. In the Google Cloud console, open **Google Auth Platform** (older projects label it **APIs & Services → OAuth consent
   screen**).
2. Under **Branding**, set the app name, a support email, and a developer contact. These appear on the Google consent
   page.
3. Under **Audience**, choose **External** unless every member is in one Google Workspace organization that allows
   internal apps. Leave the app in production after the next step, not in **Testing**.
4. Under **Data access**, add only these scopes: `openid`, `email`, `profile`. Remove anything else. These basic profile
   scopes do not need Google's verification review.
5. Under **Audience**, choose **Publish app** and confirm. Testing mode caps the number of users and is not meant for
   real use. Publishing keeps ordinary Google accounts working; the app's own allow list still decides who gets in.

### 2. Create the Web client

1. Open **Clients**, then **Create client**.
2. Choose **Web application** and give it a name such as `parlay-club web`.
3. Leave **Authorized JavaScript origins** empty. The app uses a server-side redirect, not browser JavaScript.
4. Under **Authorized redirect URIs**, add both values exactly:

   ```text
   http://localhost:8080/auth/google/callback
   https://SERVICE_URL/auth/google/callback
   ```

   Replace `SERVICE_URL` with the Cloud Run URL or custom domain. Use `https` in prod and no trailing slash.

   If you change the local port, add the matching URI. `localhost` and `127.0.0.1` are different hosts to Google, so
   register the exact one you use.

5. Create the client and copy the **Client ID** and **Client secret**. The secret is shown once; if you lose it, create a
   new client secret on the same client.

The app builds the callback from `BASE_URL + /auth/google/callback`. A mismatch shows `redirect_uri_mismatch` at Google,
before the app sees the request.

### 3. Set the secrets

The deploy setup normally creates the secrets empty. If your project does not have them yet, create them first:

```sh
gcloud secrets create google-client-id --replication-policy=automatic
gcloud secrets create google-client-secret --replication-policy=automatic
```

Then add a version with each copied value:

```sh
printf '%s' "$GOOGLE_CLIENT_ID" | gcloud secrets versions add google-client-id --data-file=-
printf '%s' "$GOOGLE_CLIENT_SECRET" | gcloud secrets versions add google-client-secret --data-file=-
```

Keep `session-key` and `odds-api-key` as they are. `SESSION_KEY` signs the session and round-trip cookies; changing it
signs every browser out.

### 4. Set the prod environment

Cloud Run needs the four secret values plus the public base URL and the email allow list. The allow list is
configuration, not a secret.

```text
GOOGLE_CLIENT_ID     from secret google-client-id
GOOGLE_CLIENT_SECRET from secret google-client-secret
BASE_URL             https://SERVICE_URL
ALLOWED_EMAILS       friend@example.com,other@example.com=Friend
SESSION_KEY          from secret session-key
ODDS_API_KEY         from secret odds-api-key
```

`BASE_URL` must match the redirect URI's scheme, host, and port exactly. `ALLOWED_EMAILS` is a comma list of `email` or
`email=Name` entries. Entries are trimmed and compared without case. Prod refuses to start with an empty list, so add at
least one address before deploying.

The `=Name` form is only for people whose users came from the history import. It links that email to an unlinked user
with that name on first sign-in, and the user keeps the stored name and avatar color. A normal first-time user needs no
`=Name`; leave it off and the app creates a user from the Google profile.

If your deploy workflow owns the `gcloud run deploy` flags, update it there:

```sh
gcloud run deploy parlay-club ... \
  --set-env-vars APP_ENV=prod,DB_PATH=/data/parlay.db,LITESTREAM_BUCKET=BUCKET,BASE_URL=https://SERVICE_URL,ALLOWED_EMAILS="friend@example.com,other@example.com=Friend" \
  --set-secrets SESSION_KEY=session-key:latest,GOOGLE_CLIENT_ID=google-client-id:latest,GOOGLE_CLIENT_SECRET=google-client-secret:latest,ODDS_API_KEY=odds-api-key:latest
```

### 5. Try local sign-in first

1. Start the app with the local client values:

   ```sh
   GOOGLE_CLIENT_ID="..." \
   GOOGLE_CLIENT_SECRET="..." \
   ALLOWED_EMAILS="you@example.com" \
   mise run dev
   ```

2. Open <http://localhost:8080/login> and choose **Sign in with Google**.
3. Sign in with an allowed account. You should land on `/` with your picture in the top bar.
4. Sign in with a second account that is not on the list. You should see the 403 page: "That Google account isn't on
   the list yet. Ask whoever runs the club to add you."

With a client ID set, `/dev/login` is not registered. Remove the Google values to get the local dev form back.

### 6. Deploy and try prod

Deploy through the normal workflow, then open `BASE_URL/login`. Sign in with an allowed account and confirm the
callback returns to `/`. Sign-in adds a little time on a Cloud Run cold start; the callback may wait a second.

## Rollback

- **Revoke the app's access to Google:** delete the OAuth client, or remove the callback URI from the client. New
  sign-ins stop; existing sessions keep working until they expire or the gate refuses them.
- **Roll back the app:** send traffic to the previous Cloud Run revision. The previous revision used the old auth
  cookies, so users sign in again.
- **Undo a bad deploy:** restore the previous `BASE_URL` and `ALLOWED_EMAILS` values and redeploy. A bad client secret
  needs a new secret version from the Google client, then a new revision.
- **Locked out:** add your email to `ALLOWED_EMAILS` and redeploy. A malformed entry or an empty list in prod fails
  startup and prints the reason.
- **Invalidate every session:** add a new `session-key` version and redeploy. Everyone signs in again.
- **Turn one person off:** remove their email from `ALLOWED_EMAILS` and redeploy, or set their `users.active` to 0. Their
  next request goes to `/login`, and their history stays.

## Verify

- `GET $BASE_URL/login` shows the Google button and no dev link in prod.
- A first sign-in with an allowed account lands on `/`, sets `pc_session`, and shows the Google picture.
- A disallowed account gets a 403 on the sign-in page, and no `pc_session` cookie is set.
- Removing an allowed email and redeploying sends that person's next page load to `/login`.
- `GET $BASE_URL/auth/google/callback` without a `pc_oauth` cookie returns the sign-in page with 400, not a server error.
- At phone width, the login page and the 403 page fit without horizontal scrolling.
- Cloud Run logs show no startup config error and no `redirect_uri_mismatch` in the Google error page.

## Troubleshooting

| Symptom                                     | Likely cause and fix                                                        |
| ------------------------------------------- | --------------------------------------------------------------------------- |
| `redirect_uri_mismatch`                     | `BASE_URL`, scheme, host, port, or path differs from the registered URI.    |
| `invalid_client`                            | Wrong client ID or secret, or the client was deleted. Recheck the secret.   |
| "That Google account isn't on the list yet" | The email is not in `ALLOWED_EMAILS`, or it has a typo or extra spaces.     |
| "Sign-in didn't work. Try again."           | The round trip expired, the state did not match, or the token was refused.  |
| Consent screen shows "unverified app"       | Extra or sensitive scopes were added. Keep only `openid email profile`.     |
| A friend cannot sign in while tests pass    | The app is still in Testing. Publish it to production under Audience.       |
| Works locally, fails in prod                 | The prod redirect URI is missing, or `BASE_URL` uses the wrong Cloud Run URL. |
