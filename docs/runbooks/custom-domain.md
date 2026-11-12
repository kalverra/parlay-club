---
id: custom-domain
title: Custom domain with Cloudflare
type: runbook
summary: Route a custom subdomain to Cloud Run through Cloudflare with origin header rewriting and OAuth callback configuration.
tags: [custom-domain, cloudflare, cloud-run, dns, oauth, runbook]
sources: [internal/config/config.go, internal/auth/provider.go, .github/workflows/deploy.yml, deploy/setup.sh]
last_verified: 2026-11-12
---

# Custom domain with Cloudflare

## Purpose

Expose the Cloud Run service on a branded custom subdomain (such as `parlay.example.com`) through Cloudflare.

The app requires root-level mounting (`/`). All template links, static asset paths (`/static/...`), HTMX endpoints,
redirects, and session cookies assume the app lives at `/`. Using a dedicated subdomain satisfies this requirement
without rewriting application code.

## Preconditions

- The app is deployed to Cloud Run and has a working `.run.app` service URL (for example,
  `parlay-club-424437296088.us-central1.run.app`).
- The parent domain is active in Cloudflare with SSL/TLS encryption set to **Full (strict)**.
- Admin access to the GitHub repository to set Actions variables.
- Permission to edit OAuth 2.0 Client credentials in Google Cloud Console.

## Steps

### 1. Create the Cloudflare DNS record

1. Log in to the Cloudflare dashboard and select the parent domain.
2. Go to **DNS → Records → Add record**.
3. Configure the CNAME:
   - **Type:** `CNAME`
   - **Name:** `parlay` (or your chosen subdomain prefix)
   - **Target:** The assigned Cloud Run hostname (e.g. `SERVICE-PROJECT_NUMBER.REGION.run.app`, without `https://`)
   - **Proxy status:** **Proxied** (orange cloud enabled)
   - **TTL:** Auto
4. Save the record.

### 2. Configure the Cloudflare Origin Rule

Cloud Run validates the HTTP `Host` header on incoming requests. If the header contains the custom domain instead of the
`.run.app` hostname, Cloud Run returns HTTP 404. An Origin Rule rewrites the header before traffic hits Cloud Run.

1. In Cloudflare, navigate to **Rules → Origin Rules**.
2. Click **Create rule**.
3. Name the rule: `Cloud Run Host Rewrite`.
4. Under **When incoming requests match...**, select **Custom filter expression**:
   - **Field:** `Hostname`
   - **Operator:** `equals`
   - **Value:** `parlay.example.com`
5. Under **Host Header**, select **Rewrite to...**:
   - Choose **Override to single value**.
   - Enter your Cloud Run hostname (e.g. `SERVICE-PROJECT_NUMBER.REGION.run.app`).
6. Click **Deploy**.

### 3. Update the BASE_URL repository variable

The app reads `BASE_URL` to construct OAuth callback redirects, session cookie scopes, and absolute links.

From the repository clone, set the updated URL:

```sh
gh variable set BASE_URL --body "https://parlay.example.com"
```

Trigger a deploy so Cloud Run restarts with the new `BASE_URL`:

```sh
gh workflow run deploy.yml
```

### 4. Register the Google OAuth callback

Google rejects sign-in attempts if the redirect URI is not pre-registered.

1. Open the [Google Cloud Console Credentials](https://console.cloud.google.com/apis/credentials) page for the project.
2. Under **OAuth 2.0 Client IDs**, select the Web client used by the app.
3. Under **Authorized redirect URIs**, add the custom domain callback:

   ```text
   https://parlay.example.com/auth/google/callback
   ```

4. Keep the direct `.run.app` callback in the list as a fallback:

   ```text
   https://SERVICE-PROJECT_NUMBER.REGION.run.app/auth/google/callback
   ```

5. Click **Save**.

## Rollback

If traffic fails or the custom domain has issues:

1. Revert `BASE_URL` in GitHub variables:

   ```sh
   gh variable set BASE_URL --body "https://SERVICE-PROJECT_NUMBER.REGION.run.app"
   ```

2. Redeploy the service.
3. Share the direct `.run.app` URL with users while debugging Cloudflare rules.

## Verify

1. In a terminal, verify DNS resolution and HTTPS handshake:

   ```sh
   curl -sI https://parlay.example.com/healthz
   ```

   Expect an `HTTP/2 200` response.
2. Open `https://parlay.example.com` in a browser.
3. Confirm static assets load correctly from `/static/app.css` and `/static/htmx.min.js`.
4. Click **Sign in with Google**.
   - Verify the Google consent screen appears.
   - Complete sign-in and confirm you land back on `/groups` or `/join` with an active session cookie (`pc_session`).
