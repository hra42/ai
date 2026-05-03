// Cloudflare Worker that serves the `ai` install script at ai.hra42.com/install.
//
// Pass-through fetches the install.sh from the latest GitHub release so the URL
// stays stable across releases and we can give nicer errors than a raw 404.

const REPO = "hra42/ai";
const SOURCE = `https://github.com/${REPO}/releases/latest/download/install.sh`;
const RELEASES_PAGE = `https://github.com/${REPO}/releases`;

const HTML_LANDING = `<!doctype html>
<meta charset="utf-8">
<title>ai — install</title>
<style>
  body { font: 16px/1.5 system-ui, sans-serif; max-width: 40rem; margin: 4rem auto; padding: 0 1rem; color: #1a1a1a; }
  code, pre { font-family: ui-monospace, monospace; background: #f4f4f4; padding: 0.1em 0.3em; border-radius: 3px; }
  pre { padding: 1rem; overflow-x: auto; }
  a { color: #2855c8; }
</style>
<h1>Install <code>ai</code></h1>
<p>This URL serves a shell install script. Run it like this:</p>
<pre>curl -fsSL https://ai.hra42.com/install | sh</pre>
<p>Source: <a href="https://github.com/${REPO}/blob/main/scripts/install.sh">scripts/install.sh</a> · Releases: <a href="${RELEASES_PAGE}">github.com/${REPO}/releases</a></p>
`;

function plain(body, status = 200, extra = {}) {
	return new Response(body, {
		status,
		headers: {
			"content-type": "text/plain; charset=utf-8",
			"cache-control": "public, max-age=60",
			...extra,
		},
	});
}

export default {
	async fetch(request) {
		const url = new URL(request.url);

		if (url.pathname !== "/install") {
			return plain(`not found\n\nsee ${RELEASES_PAGE}\n`, 404);
		}

		// Friendly landing page for browsers; raw script for curl/wget/etc.
		const accept = request.headers.get("accept") || "";
		const ua = request.headers.get("user-agent") || "";
		const isBrowser = accept.includes("text/html") && !/curl|wget|fetch/i.test(ua);
		if (isBrowser) {
			return new Response(HTML_LANDING, {
				headers: {
					"content-type": "text/html; charset=utf-8",
					"cache-control": "public, max-age=300",
				},
			});
		}

		let upstream;
		try {
			upstream = await fetch(SOURCE, {
				redirect: "follow",
				cf: { cacheTtl: 60, cacheEverything: true },
			});
		} catch (e) {
			return plain(
				`upstream fetch failed: ${e.message}\n\nsee ${RELEASES_PAGE}\n`,
				502,
			);
		}

		if (upstream.status === 404) {
			return plain(
				`no install script in the latest release of ${REPO} yet.\n` +
					`this usually means no release has been published, or the release is missing install.sh.\n` +
					`see ${RELEASES_PAGE}\n`,
				503,
			);
		}

		if (!upstream.ok) {
			return plain(
				`upstream returned ${upstream.status} ${upstream.statusText}\n\nsee ${RELEASES_PAGE}\n`,
				502,
			);
		}

		// Pass through the script body with our own cache headers.
		return new Response(upstream.body, {
			status: 200,
			headers: {
				"content-type": "text/x-shellscript; charset=utf-8",
				"cache-control": "public, max-age=300",
				"x-source": SOURCE,
			},
		});
	},
};
