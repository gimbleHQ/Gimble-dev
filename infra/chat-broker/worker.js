const USER_RE = /^[a-z0-9][a-z0-9_-]{1,39}$/;
const SESSION_RE = /^[a-f0-9]{8,32}$/;

function isValidTunnelURL(raw) {
  try {
    const u = new URL(String(raw || ""));
    return u.protocol === "https:" && (u.hostname.endsWith(".trycloudflare.com") || u.hostname.endsWith(".gimble.dev"));
  } catch {
    return false;
  }
}

function randomNonce() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function verifyTunnelOwnership(tunnelUrl) {
  const nonce = randomNonce();
  const proofURL = `${String(tunnelUrl).replace(/\/$/, "")}/__gimble_proof?nonce=${encodeURIComponent(nonce)}`;
  const resp = await fetch(proofURL, {
    method: "GET",
    redirect: "follow",
    headers: {
      accept: "text/plain",
      "user-agent": "Mozilla/5.0 (Gimble Broker; +https://gimble.dev)",
    },
    cf: { cacheTtl: 0, cacheEverything: false },
  });
  if (!resp.ok) return false;
  const text = (await resp.text()).trim();
  return text === nonce;
}

function parseTTL(rawTTL) {
  return Math.max(60, Math.min(24 * 3600, Number(rawTTL || 6 * 3600)));
}

function parseRegisterPayload(payload) {
  if (!payload) {
    throw new Error("invalid payload");
  }

  const username = String(payload.username || "").toLowerCase();
  const sessionId = String(payload.session_id || "").toLowerCase();
  const tunnelUrl = String(payload.tunnel_url || "");
  const ttl = parseTTL(payload.expires_in_seconds);

  if (!USER_RE.test(username) || !SESSION_RE.test(sessionId) || !isValidTunnelURL(tunnelUrl)) {
    throw new Error("invalid registration fields");
  }

  return {
    key: `${username}/${sessionId}`,
    username,
    sessionId,
    tunnelUrl,
    ttl,
    createdAt: payload.created_at || new Date().toISOString(),
  };
}

async function doPutSession(env, key, value, ttl) {
  if (env.SESSION_BROKER) {
    const id = env.SESSION_BROKER.idFromName("broker");
    const stub = env.SESSION_BROKER.get(id);
    const resp = await stub.fetch("https://do/session/put", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ key, value, ttl }),
    });
    if (!resp.ok) {
      throw new Error(await resp.text());
    }
    return;
  }

  if (!env.SESSIONS) {
    throw new Error("no session store configured");
  }
  await env.SESSIONS.put(key, JSON.stringify(value), { expirationTtl: ttl });
}

async function doGetSession(env, key) {
  if (env.SESSION_BROKER) {
    const id = env.SESSION_BROKER.idFromName("broker");
    const stub = env.SESSION_BROKER.get(id);
    const resp = await stub.fetch(`https://do/session/get?key=${encodeURIComponent(key)}`);
    if (resp.status === 404) return null;
    if (!resp.ok) throw new Error(await resp.text());
    return await resp.json();
  }

  if (!env.SESSIONS) {
    return null;
  }
  const raw = await env.SESSIONS.get(key);
  if (!raw) return null;
  return JSON.parse(raw);
}

function parseUnregisterPayload(payload) {
  if (!payload) {
    throw new Error("invalid payload");
  }

  const username = String(payload.username || "").toLowerCase();
  const sessionId = String(payload.session_id || "").toLowerCase();
  if (!USER_RE.test(username) || !SESSION_RE.test(sessionId)) {
    throw new Error("invalid unregister fields");
  }

  return { key: `${username}/${sessionId}` };
}

async function doDeleteSession(env, key) {
  if (env.SESSION_BROKER) {
    const id = env.SESSION_BROKER.idFromName("broker");
    const stub = env.SESSION_BROKER.get(id);
    const resp = await stub.fetch("https://do/session/delete", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ key }),
    });
    if (!resp.ok) throw new Error(await resp.text());
    return;
  }

  if (env.SESSIONS) {
    await env.SESSIONS.delete(key);
  }
}

export class SessionBroker {
  constructor(state) {
    this.state = state;
  }

  async fetch(request) {
    const url = new URL(request.url);

    if (request.method === "POST" && url.pathname === "/session/put") {
      const payload = await request.json().catch(() => null);
      if (!payload || !payload.key || !payload.value) {
        return new Response("invalid payload", { status: 400 });
      }
      const ttl = parseTTL(payload.ttl);
      const expiresAt = Date.now() + ttl * 1000;
      const value = {
        ...payload.value,
        expires_at_ms: expiresAt,
      };
      await this.state.storage.put(String(payload.key), value);
      return Response.json({ ok: true });
    }

    if (request.method === "GET" && url.pathname === "/session/get") {
      const key = String(url.searchParams.get("key") || "");
      if (!key) {
        return new Response("missing key", { status: 400 });
      }

      const value = await this.state.storage.get(key);
      if (!value) {
        return new Response("session not found", { status: 404 });
      }

      if (Number(value.expires_at_ms || 0) > 0 && Date.now() > Number(value.expires_at_ms)) {
        await this.state.storage.delete(key);
        return new Response("session not found", { status: 404 });
      }

      return Response.json(value);
    }


    if (request.method === "POST" && url.pathname === "/session/delete") {
      const payload = await request.json().catch(() => null);
      if (!payload || !payload.key) {
        return new Response("invalid payload", { status: 400 });
      }
      await this.state.storage.delete(String(payload.key));
      return Response.json({ ok: true });
    }
    return new Response("not found", { status: 404 });
  }
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (request.method === "POST" && url.pathname === "/api/unregister") {
      const payload = await request.json().catch(() => null);
      let parsed;
      try {
        parsed = parseUnregisterPayload(payload);
      } catch (err) {
        return new Response(String(err.message || "invalid payload"), { status: 400 });
      }

      await doDeleteSession(env, parsed.key);
      return Response.json({ ok: true, key: parsed.key });
    }

    if (request.method === "POST" && url.pathname === "/api/register") {
      const payload = await request.json().catch(() => null);
      let parsed;
      try {
        parsed = parseRegisterPayload(payload);
      } catch (err) {
        return new Response(String(err.message || "invalid payload"), { status: 400 });
      }

      const ok = await verifyTunnelOwnership(parsed.tunnelUrl);
      if (!ok) {
        return new Response("tunnel proof failed", { status: 403 });
      }

      await doPutSession(
        env,
        parsed.key,
        {
          tunnel_url: parsed.tunnelUrl,
          created_at: parsed.createdAt,
        },
        parsed.ttl
      );

      return Response.json({ ok: true, key: parsed.key, public_url: `https://${url.host}/${parsed.key}` });
    }

    const parts = url.pathname.replace(/^\/+/, "").split("/");
    if (parts.length < 2 || !parts[0] || !parts[1]) {
      return new Response("not found", { status: 404 });
    }

    const sessionKey = `${parts[0]}/${parts[1]}`;
    const record = await doGetSession(env, sessionKey);
    if (!record) {
      return new Response("session not found", { status: 404 });
    }

    const upstreamBase = String(record.tunnel_url || "").replace(/\/$/, "");
    if (!upstreamBase.startsWith("https://")) {
      return new Response("invalid upstream", { status: 502 });
    }

    const rest = parts.slice(2).join("/");
    const upstreamUrl = `${upstreamBase}/${rest}${url.search}`;
    const upstreamReq = new Request(upstreamUrl, request);
    upstreamReq.headers.set("host", new URL(upstreamBase).host);

    return fetch(upstreamReq);
  },
};
