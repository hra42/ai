# Cloudflare Worker — `ai.hra42.com/install`

Pass-through worker that serves [`scripts/install.sh`](../scripts/install.sh) from the latest GitHub release of `hra42/ai`.

- `/install` (curl/wget) → fetches `releases/latest/download/install.sh` and pipes it through with `content-type: text/x-shellscript`
- `/install` (browser) → small HTML landing page with the curl one-liner
- No release yet / asset missing → `503` with a plaintext explanation
- Upstream error → `502` with a plaintext explanation
- Any other path → `404` linking to the releases page

## Deploy (one-time setup)

This worker is deployed via Cloudflare's GitHub integration — every push to `main` that touches `cloudflare/**` redeploys automatically. No `wrangler` CLI or API tokens needed.

1. Cloudflare Dashboard → **Workers & Pages** → **Create** → **Connect to Git**.
2. Authorize the Cloudflare GitHub App on `hra42/ai` (read-only is fine).
3. Configure:
   - **Project name**: `ai`
   - **Production branch**: `main`
   - **Root directory**: `cloudflare`
   - **Build command**: *(leave empty)*
   - **Deploy command**: `npx wrangler deploy`
4. Save and deploy.
5. Confirm the route `ai.hra42.com/install` is attached (Workers → `ai` → Settings → Triggers → Routes). The route is declared in `wrangler.toml` and gets created on first deploy; the `hra42.com` zone must already exist in this Cloudflare account.

After setup, edits to `install-worker.js` ship on push to `main`.

## Local dev

```sh
cd cloudflare
npx wrangler dev
```

Then `curl localhost:8787/install`.
