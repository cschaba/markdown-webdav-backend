// Checks in a real browser that the export's settings apply by themselves, are
// remembered, and that Print never prints a page whose settings are stale.
// None of that can be told from the HTML.
//
//   ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18086 -vault test=./testdata/vault,nogit &
//   chromium --headless=new --remote-debugging-port=9333 --user-data-dir=$(mktemp -d) about:blank &
//   node tools/check-export-settings.mjs 9333 http://127.0.0.1:18086
//
// Exits 1 if a check fails.
const [port, base] = [process.argv[2], process.argv[3]];
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
for (let i = 0; i < 50 && !target; i++) {
  try { target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find(t => t.type === "page"); } catch { await sleep(200); }
}
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise(r => ws.addEventListener("open", r));
let id = 0; const pending = new Map();
ws.addEventListener("message", e => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result || m.error); pending.delete(m.id); } });
const send = (method, params = {}) => new Promise(r => { pending.set(++id, r); ws.send(JSON.stringify({ id, method, params })); });
const evaluate = async expr => (await send("Runtime.evaluate", { expression: expr, returnByValue: true })).result.value;
const key = async (k, code, text) => { for (const type of ["keyDown", "keyUp"]) await send("Input.dispatchKeyEvent", { type, key: k, code, text: type === "keyDown" ? text : undefined, windowsVirtualKeyCode: { Tab: 9, Backspace: 8 }[k] }); };
const click = async sel => {
  const p = await evaluate(`(() => { const r = document.querySelector(${JSON.stringify(sel)}).getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
  for (const type of ["mouseMoved", "mousePressed", "mouseReleased"]) await send("Input.dispatchMouseEvent", { type, x: p.x, y: p.y, button: "left", clickCount: 1 });
};
const state = () => evaluate(`JSON.stringify({ url: location.search, size: document.querySelector("select[name=size]").value, fields: [...document.querySelectorAll("form.export-bar [name]")].map(f => f.name).join(","), sub: document.querySelector("input[name=sub]").checked, paper: document.querySelector(".paper").style.width, docs: document.querySelectorAll("section.doc:not(.contents)").length, applyVisible: !!document.querySelector("form.export-bar button[type=submit]:not(noscript *)") })`).then(JSON.parse);

let failures = 0;
const check = (what, ok, detail) => { console.log((ok ? "ok    " : "FAIL  ") + what + (ok ? "" : "  -> " + detail)); if (!ok) failures++; };

await send("Page.enable");
// Record every print(): which page it printed, and with which @page rule.
await send("Page.addScriptToEvaluateOnNewDocument", { source: `window.print = function () { var log = JSON.parse(sessionStorage.getItem("prints") || "[]"); log.push({ url: location.search, css: (document.querySelector("head style") || {}).textContent }); sessionStorage.setItem("prints", JSON.stringify(log)); };` });
const prints = async () => JSON.parse(await evaluate(`sessionStorage.getItem("prints") || "[]"`));
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 800, deviceScaleFactor: 1, mobile: false });
await send("Page.navigate", { url: `${base}/test/-/export?path=10%20Printing.md` });
await sleep(900);
let s = await state();
check("starts with the defaults", s.size === "A4" && !s.sub && s.docs === 1, JSON.stringify(s));
check("no Apply button to be seen", !s.applyVisible, "a submit button is visible");
check("no page number setting: it was removed", s.fields === "path,set,size,sub", s.fields);

// a choice applies at once
const choose = size => evaluate(`(() => { const f = document.querySelector("select[name=size]"); f.value = ${JSON.stringify(size)}; f.dispatchEvent(new Event("change", { bubbles: true })); })()`);
await choose("A5");
await sleep(1100);
s = await state();
check("choosing a page size applies it", s.size === "A5" && s.paper === "148mm", JSON.stringify(s));
await click("input[name=sub]");
await sleep(1100);
s = await state();
check("ticking sub pages applies it", s.sub && s.docs === 5 && s.size === "A5", JSON.stringify(s));

// the next export, reached the way a footer link reaches it, remembers both
await send("Page.navigate", { url: `${base}/test/-/export?path=09%20Embedded%20notes.md` });
await sleep(900);
s = await state();
check("the next export remembers size and sub pages", s.size === "A5" && s.sub, JSON.stringify(s));

// the place in a long preview survives applying a setting
await send("Page.navigate", { url: `${base}/test/-/export?path=10%20Printing.md&set=1&sub=1&size=A5` });
await sleep(900);
await evaluate(`scrollTo(0, 1500)`);
await sleep(200);
await choose("B5");
await sleep(1300);
const y = await evaluate(`scrollY`);
s = await state();
check("the scroll position is kept", s.size === "B5" && Math.abs(y - 1500) < 5, `size ${s.size}, scrollY ${y}`);

// Print while a setting is not applied yet - changed, the page not back yet.
// It must print the page with the setting, once. (With a number field this was
// one click: it left the field, which applied it, and printed the old page.)
await send("Page.navigate", { url: `${base}/test/-/export?path=10%20Printing.md&set=1&size=A4` });
await sleep(900);
await evaluate(`sessionStorage.removeItem("prints")`);
await evaluate(`document.querySelector("select[name=size]").value = "A5"`); // changed, not yet applied
await click("#print");
await sleep(2500);
let log = await prints();
check("Print with an unapplied setting prints once", log.length === 1, JSON.stringify(log));
check("... the page that has the setting", log.length > 0 && /size: A5/.test(log[0].css), JSON.stringify(log));
check("... and a reload would not print again", !(await evaluate(`location.search`)).includes("print="), await evaluate(`location.search`));

// With nothing to apply, Print just prints.
await evaluate(`sessionStorage.removeItem("prints")`);
await click("#print");
await sleep(600);
log = await prints();
check("with everything applied, Print prints at once", log.length === 1 && /size: A5/.test(log[0].css), JSON.stringify(log));

ws.close();
console.log(failures ? `\n${failures} check(s) failed` : "\nall checks passed");
process.exit(failures ? 1 : 0);
