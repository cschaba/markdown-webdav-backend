// Loads the pages in a real browser under their Content-Security-Policy and
// checks that nothing the pages need is refused by it: the policy, and the
// hashes of the scripts from the CDN, fail in the browser and nowhere else. A
// diagram that is not drawn, a graph that stays empty - the Go tests would not
// see it. Also checks that a script in the page, which the policy is there to
// stop, is stopped.
//
//   ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18086 -vault test=./testdata/vault,nogit &
//   chromium --headless=new --remote-debugging-port=9333 --user-data-dir=$(mktemp -d) about:blank &
//   node tools/check-security.mjs 9333 http://127.0.0.1:18086
//
// Needs the network: the scripts come from the CDN. Exits 1 if a check fails.
const [port, base] = [process.argv[2], process.argv[3]];
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
// A browser starting cold on a CI runner can take its time; and one that is
// up but has no page yet must be waited for too, not asked 50 times at once.
for (const deadline = Date.now() + 60000; !target && Date.now() < deadline; ) {
  try { target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find(t => t.type === "page"); } catch { /* not listening yet */ }
  if (!target) await sleep(200);
}
if (!target) { console.error(`no browser page on port ${port} after 60 s`); process.exit(2); }
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise(r => ws.addEventListener("open", r));
let id = 0; const pending = new Map(); let refused = [];
ws.addEventListener("message", e => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result || m.error); pending.delete(m.id); }
  // what the browser refuses - by the policy, or for a wrong hash - it says in its log
  if (m.method === "Log.entryAdded" && /Content Security Policy|integrity|Refused/i.test(m.params.entry.text)) refused.push(m.params.entry.text.slice(0, 200));
});
const send = (method, params = {}) => new Promise(r => { pending.set(++id, r); ws.send(JSON.stringify({ id, method, params })); });
const js = async expr => (await send("Runtime.evaluate", { expression: expr, returnByValue: true, awaitPromise: true })).result.value;
const open = async (path, wait = 1200) => { refused = []; await send("Page.navigate", { url: base + path }); await sleep(wait); };

let failures = 0;
const check = (what, ok, detail) => { console.log((ok ? "ok    " : "FAIL  ") + what + (ok ? "" : "  -> " + detail)); if (!ok) failures++; };

await send("Page.enable"); await send("Log.enable");
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 700, deviceScaleFactor: 1, mobile: false });

const pages = [
  ["a folder", "/test/"], ["a note", "/test/01%20Formatting"], ["the tags", "/test/-/tags"],
  ["a search", "/test/-/search?q=test"], ["an export", "/test/-/export?path=01%20Formatting.md"],
];
for (const [name, path] of pages) {
  await open(path, 600);
  check(name + ": nothing is refused", refused.length === 0, refused.join(" | "));
}
await open("/test/01%20Formatting", 600);
check("our own scripts run under the policy", await js(`!document.getElementById("keys-button").hidden`), "the help button stayed hidden: keys.js did not run");

const drawn = `document.querySelectorAll(".mermaid svg").length`;
await open("/test/02%20Code%20and%20Diagrams", 4000);
check("a note's diagram is drawn, by the pinned script", await js(drawn) > 0 && refused.length === 0, await js(drawn) + " drawn; " + refused.join(" | "));
check("the script carries its hash", await js(`[...document.scripts].filter(s => /^https:/.test(s.src)).every(s => s.integrity.startsWith("sha384-") && s.crossOrigin === "anonymous")`), "no integrity");
await open("/test/-/slides?path=12%20Slides.md", 4000);
check("the diagrams of a slide show are drawn", await js(drawn) > 0 && refused.length === 0, await js(drawn) + " drawn; " + refused.join(" | "));
await open("/test/-/export?path=12%20Slides.md&format=slides", 4000);
check("and those of its PDF", await js(drawn) > 0 && refused.length === 0, await js(drawn) + " drawn; " + refused.join(" | "));
await open("/test/-/graph", 3000);
check("the graph is drawn", await js(`!!document.querySelector("canvas")`) && refused.length === 0, refused.join(" | "));

// What the policy is for: script that got into a page does not run.
await open("/test/01%20Formatting", 600);
await js(`new Promise(done => { const s = document.createElement("script"); s.textContent = "window.__inline = true"; document.body.append(s);
  const e = document.createElement("script"); e.src = "https://cdn.jsdelivr.net/npm/lodash@4.17.21/lodash.min.js"; e.onload = e.onerror = () => done(); document.body.append(e); setTimeout(done, 3000); })`);
check("an inline script does not run", await js(`window.__inline !== true`), "it ran");
check("nor another script from the same CDN", await js(`typeof window._ === "undefined"`), "it ran");

ws.close();
console.log(failures ? `\n${failures} check(s) failed` : "\nall checks passed");
process.exit(failures ? 1 : 0);
