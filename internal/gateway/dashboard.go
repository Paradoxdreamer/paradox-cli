package gateway

import (
	"net/http"
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Paradox Cloud</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
    background: #0a0a0a;
    color: #e8e8e8;
    min-height: 100vh;
    padding: 2rem 1.5rem 3rem;
    line-height: 1.45;
  }
  h1 {
    font-size: 1.1rem;
    font-weight: 600;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    border-bottom: 1px solid #333;
    padding-bottom: 0.75rem;
    margin-bottom: 1.5rem;
  }
  h1 span { color: #888; font-weight: 400; letter-spacing: 0.04em; text-transform: none; }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
    margin-bottom: 2rem;
  }
  .card {
    border: 1px solid #2a2a2a;
    background: #111;
    padding: 1rem 1.1rem;
  }
  .card .label {
    font-size: 0.7rem;
    color: #777;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    margin-bottom: 0.4rem;
  }
  .card .value { font-size: 1.5rem; font-weight: 600; color: #f0f0f0; }
  .card .sub { font-size: 0.75rem; color: #666; margin-top: 0.35rem; }
  .ok { color: #c8c8c8; }
  .bad { color: #888; }
  table { width: 100%; border-collapse: collapse; font-size: 0.8rem; margin-top: 0.5rem; }
  th, td { text-align: left; padding: 0.45rem 0.5rem; border-bottom: 1px solid #222; }
  th { color: #666; font-weight: 500; text-transform: uppercase; letter-spacing: 0.06em; font-size: 0.65rem; }
  a { color: #aaa; text-decoration: none; border-bottom: 1px solid #444; }
  a:hover { color: #fff; border-color: #888; }
  .footer { margin-top: 2.5rem; font-size: 0.7rem; color: #555; border-top: 1px solid #222; padding-top: 1rem; }
  .row { display: flex; gap: 1.5rem; flex-wrap: wrap; align-items: baseline; }
  button {
    background: #1a1a1a; color: #ddd; border: 1px solid #333;
    font-family: inherit; font-size: 0.75rem; padding: 0.4rem 0.75rem;
    cursor: pointer; letter-spacing: 0.04em;
  }
  button:hover { background: #222; border-color: #555; color: #fff; }
  #err { color: #999; font-size: 0.75rem; margin-top: 0.75rem; min-height: 1.2em; }
</style>
</head>
<body>
  <h1>Paradox Cloud <span>local board</span></h1>
  <div class="row" style="margin-bottom:1.25rem">
    <button type="button" id="refresh">Refresh</button>
    <span id="ts" style="color:#555;font-size:0.75rem"></span>
  </div>
  <div class="grid">
    <div class="card"><div class="label">Gateway</div><div class="value" id="gw">—</div><div class="sub" id="gw-sub">/health</div></div>
    <div class="card"><div class="label">Queue pending</div><div class="value" id="q-pending">—</div><div class="sub">waiting</div></div>
    <div class="card"><div class="label">Queue running</div><div class="value" id="q-running">—</div><div class="sub">in flight</div></div>
    <div class="card"><div class="label">Succeeded</div><div class="value" id="q-ok">—</div><div class="sub">completed jobs</div></div>
    <div class="card"><div class="label">Failed / dead</div><div class="value" id="q-bad">—</div><div class="sub">needs attention</div></div>
    <div class="card"><div class="label">Obs events</div><div class="value" id="obs-ev">—</div><div class="sub">metrics <span id="obs-met">—</span> · errors <span id="obs-err">—</span></div></div>
  </div>
  <div class="card" style="margin-bottom:1rem">
    <div class="label">Upscale jobs (recent)</div>
    <table><thead><tr><th>ID</th><th>Status</th><th>Progress</th><th>Input</th></tr></thead>
    <tbody id="up-body"><tr><td colspan="4" style="color:#555">—</td></tr></tbody></table>
  </div>
  <div class="card">
    <div class="label">Agents</div>
    <table><thead><tr><th>Name</th><th>Model</th></tr></thead>
    <tbody id="ag-body"><tr><td colspan="2" style="color:#555">—</td></tr></tbody></table>
  </div>
  <p id="err"></p>
  <div class="footer">Monochrome board · polls local API ·
    <a href="/health">/health</a> · <a href="/v1/queue/status">/v1/queue/status</a>
  </div>
<script>
async function j(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(url + " " + r.status);
  return r.json();
}
function $(id) { return document.getElementById(id); }
async function load() {
  $("err").textContent = "";
  try {
    const health = await j("/health");
    $("gw").textContent = health.status === "ok" ? "UP" : "DOWN";
    $("gw").className = "value ok";
    $("gw-sub").textContent = health.service || "paradox-gateway";
  } catch (e) {
    $("gw").textContent = "DOWN"; $("gw").className = "value bad";
  }
  try {
    const q = await j("/v1/queue/status");
    $("q-pending").textContent = q.pending ?? 0;
    $("q-running").textContent = q.running ?? 0;
    $("q-ok").textContent = q.succeeded ?? 0;
    $("q-bad").textContent = (q.failed ?? 0) + (q.dead ?? 0);
  } catch (e) { $("err").textContent += "queue: " + e.message + " "; }
  try {
    const o = await j("/v1/obs/status");
    $("obs-ev").textContent = o.events ?? 0;
    $("obs-met").textContent = o.metrics ?? 0;
    $("obs-err").textContent = o.errors ?? 0;
  } catch (e) { $("err").textContent += "obs: " + e.message + " "; }
  try {
    const ups = await j("/v1/upscale");
    const body = $("up-body"); body.innerHTML = "";
    const list = Array.isArray(ups) ? ups.slice(0, 8) : [];
    if (!list.length) body.innerHTML = '<tr><td colspan="4" style="color:#555">no jobs</td></tr>';
    else for (const u of list) {
      const tr = document.createElement("tr");
      tr.innerHTML = "<td>"+(u.id||"")+"</td><td>"+(u.status||"")+"</td><td>"+(u.progress??"")+"%</td><td>"+(u.input_bucket||"")+"/"+(u.input_key||"")+"</td>";
      body.appendChild(tr);
    }
  } catch (e) { $("up-body").innerHTML = '<tr><td colspan="4" style="color:#555">unavailable</td></tr>'; }
  try {
    const agents = await j("/v1/agent/list");
    const body = $("ag-body"); body.innerHTML = "";
    const list = Array.isArray(agents) ? agents : [];
    if (!list.length) body.innerHTML = '<tr><td colspan="2" style="color:#555">no agents</td></tr>';
    else for (const a of list) {
      const tr = document.createElement("tr");
      tr.innerHTML = "<td>"+(a.name||"")+"</td><td>"+(a.model||"")+"</td>";
      body.appendChild(tr);
    }
  } catch (e) { $("ag-body").innerHTML = '<tr><td colspan="2" style="color:#555">unavailable</td></tr>'; }
  $("ts").textContent = new Date().toISOString();
}
$("refresh").onclick = load;
load();
setInterval(load, 5000);
</script>
</body>
</html>
`

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(dashboardHTML))
}
