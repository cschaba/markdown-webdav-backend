// Presses real keys in a real browser and checks what happens. Whether "j"
// scrolls, "]]" moves the focus to a heading or "?" opens a dialog that traps
// the focus cannot be told from the HTML.
//
//   ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18086 -vault test=./testdata/vault,nogit &
//   chromium --headless=new --remote-debugging-port=9333 --user-data-dir=$(mktemp -d) about:blank &
//   node tools/check-keyboard.mjs 9333 http://127.0.0.1:18086
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
const js = async expr => (await send("Runtime.evaluate", { expression: expr, returnByValue: true })).result.value;

const NAMED = { Escape: 27, Enter: 13, Tab: 9, ArrowDown: 40, ArrowUp: 38, Backspace: 8 };
// press("j"), press("G") (shifted by itself), press("j", {ctrl: true}), press("Escape")
async function press(key, mods = {}) {
  const named = key in NAMED;
  const shifted = !named && (key !== key.toLowerCase() || "?{}".includes(key));
  const modifiers = (mods.alt ? 1 : 0) | (mods.ctrl ? 2 : 0) | (mods.meta ? 4 : 0) | (shifted || mods.shift ? 8 : 0);
  const event = { key, modifiers, windowsVirtualKeyCode: named ? NAMED[key] : key.toUpperCase().charCodeAt(0) };
  if (named) Object.assign(event, { code: key, nativeVirtualKeyCode: NAMED[key] }); // without them Enter activates nothing
  // Enter carries a character too; without it a <summary> does not toggle
  const text = key === "Enter" ? "\r" : named || mods.ctrl || mods.alt || mods.meta ? undefined : key;
  await send("Input.dispatchKeyEvent", { type: "keyDown", ...event, text });
  await send("Input.dispatchKeyEvent", { type: "keyUp", ...event });
  await sleep(60);
}
const type = async keys => { for (const k of keys) await press(k); };
const open = async path => { await send("Page.navigate", { url: base + path }); await sleep(800); };
const active = () => js(`(() => { const a = document.activeElement; return a ? (a.id ? "#" + a.id : a.className ? a.tagName.toLowerCase() + "." + a.className : a.tagName.toLowerCase()) + "|" + (a.textContent || a.value || "").trim().slice(0, 40) : ""; })()`);
const settle = () => sleep(450); // smooth scrolling

let failures = 0;
const check = (what, ok, detail) => { console.log((ok ? "ok    " : "FAIL  ") + what + (ok ? "" : "  -> " + detail)); if (!ok) failures++; };

await send("Page.enable");
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 700, deviceScaleFactor: 1, mobile: false });

console.log("--- a long note: scrolling");
await open("/test/10%20Printing/Chapter%202");
await press("j"); await settle();
const afterJ = await js("scrollY");
check("j scrolls down", afterJ > 0, afterJ);
await press("k"); await settle();
check("k scrolls back up", await js("scrollY") < afterJ, await js("scrollY"));
await press("d"); await settle();
check("d scrolls half a page", Math.abs(await js("scrollY") - 350) < 80, await js("scrollY"));
await press("G"); await settle();
const bottom = await js("scrollY");
check("G goes to the bottom", bottom > 600 && Math.abs(bottom + 700 - await js("document.documentElement.scrollHeight")) < 4, bottom);
await type("gg"); await settle();
check("gg goes to the top", await js("scrollY") === 0, await js("scrollY"));
await press("g"); await sleep(100);
check("a pending g is shown", await js(`document.querySelector(".key-pending").textContent + "|" + document.querySelector(".key-pending").hidden`) === "g|false", await js(`document.querySelector(".key-pending").outerHTML`));
await press("Escape");
check("Esc cancels it", await js(`document.querySelector(".key-pending").hidden`), "still shown");
await press("g"); await press("j"); await settle();
check("a sequence that leads nowhere falls back to its last key (g, j scrolls)", await js("scrollY") > 0, await js("scrollY"));
await type("gg"); await settle();
await press("j", { ctrl: true }); await press("j", { alt: true }); await press("j", { meta: true }); await settle();
check("with Ctrl, Alt or Meta the keys are left alone", await js("scrollY") === 0, await js("scrollY"));

console.log("--- the search field");
await press("/");
check("/ focuses the search field", (await active()).startsWith("#search-q"), await active());
await type("jGd");
check("typing in it is typing", await js(`document.activeElement.value`) === "jGd" && await js("scrollY") === 0, await js(`document.activeElement.value`));
await press("Escape");
check("Esc leaves the field", !(await active()).startsWith("#search-q"), await active());

console.log("--- headings and folds");
await open("/test/01%20Formatting");
await type("]]");
check("]] focuses the first heading, through its fold control", (await active()) === "summary|Level 1", await active());
await type("]]");
check("]] again the next", (await active()) === "summary|Tables", await active());
await type("[[");
check("[[ goes back", (await active()) === "summary|Level 1", await active());
await press("J"); await press("J");
check("J does the same as ]]", (await active()) === "summary|Task list", await active());
await press("K");
check("K the same as [[", (await active()) === "summary|Tables", await active());
await type("za");
check("za folds the focused section", await js(`document.activeElement.parentElement.open`) === false, "still open");
await type("za");
check("za unfolds it", await js(`document.activeElement.parentElement.open`) === true, "still closed");
await type("zM");
check("zM folds all", await js(`[...document.querySelectorAll("article details.fold")].every(d => !d.open)`), "some open");
check("the focus moves to the section that is left to see", (await active()) === "summary|Level 1", await active());
await type("zR");
check("zR unfolds all", await js(`[...document.querySelectorAll("article details.fold")].every(d => d.open)`), "some closed");
await press("Enter");
check("Enter on the focused heading folds it: the focus is real", await js(`document.activeElement.parentElement.open`) === false, "still open");

console.log("--- a list");
await open("/test/");
await press("j");
const first = await active();
check("j focuses the first item", first.startsWith("a|"), first);
await press("j");
const second = await active();
check("j again the next", second.startsWith("a|") && second !== first, second);
await press("k");
check("k goes back", await active() === first, await active());
await press("G");
const last = await active();
await type("gg");
check("G and gg go to the last and first item", last !== first && await active() === first, last);
check("the focused item can be seen", await js(`getComputedStyle(document.activeElement).outlineStyle !== "none"`), await js(`getComputedStyle(document.activeElement).outline`));
await press("G"); await press("Enter"); await sleep(800);
check("Enter opens it", (await js("location.pathname")).includes("v1.2"), await js("location.pathname"));
await press("H"); await sleep(800);
check("H goes back", await js("location.pathname") === "/test/", await js("location.pathname"));

console.log("--- going places");
await open("/test/10%20Printing/Chapter%201");
await press("-"); await sleep(800);
check("- goes up, to the folder", await js("location.pathname") === "/test/10%20Printing/", await js("location.pathname"));
await type("gt"); await sleep(800);
check("gt goes to the tags", await js("location.pathname") === "/test/-/tags", await js("location.pathname"));
await type("gh"); await sleep(800);
check("gh goes home", await js("location.pathname") === "/test/", await js("location.pathname"));

console.log("--- link hints");
await open("/test/00%20Index");
await press("f");
const hints = JSON.parse(await js(`JSON.stringify((() => { const layer = document.querySelector(".key-hints"); if (!layer) return null; const tag = [...layer.children].find(s => s.dataset.href === "/test/01%20Formatting"); return { count: layer.children.length, hidden: layer.getAttribute("aria-hidden"), label: tag && tag.textContent }; })())`));
check("f labels the links in view, for the eye only", hints && hints.count > 5 && hints.hidden === "true" && !!hints.label, JSON.stringify(hints));
await press("Escape");
check("Esc takes the labels away", await js(`!document.querySelector(".key-hints")`), "still there");
await press("f");
await type(hints.label); await sleep(800);
check("typing a label follows that link", await js("location.pathname") === "/test/01%20Formatting", await js("location.pathname"));

console.log("--- the help");
await press("?"); await sleep(200);
const help = JSON.parse(await js(`JSON.stringify({ open: document.getElementById("keys-help").open, modal: document.getElementById("keys-help").matches(":modal"), rows: document.querySelectorAll("#keys-help tr").length, inside: !!document.activeElement.closest("#keys-help"), slash: [...document.querySelectorAll("#keys-help kbd")].some(k => k.textContent === "/") })`));
check("? opens the help, as a modal dialog with the focus in it", help.open && help.modal && help.inside, JSON.stringify(help));
check("it lists the keys", help.rows >= 18 && help.slash, JSON.stringify(help));
const before = await js("scrollY");
await press("j"); await settle();
check("while it is open the keys rest", await js("scrollY") === before, await js("scrollY"));
// A modal dialog makes the rest of the page inert: Tab goes through the
// dialog and on to the browser's own controls (the page then reports <body>),
// never to a link behind it.
const visited = [];
for (let i = 0; i < 8; i++) { await press("Tab"); visited.push(await js(`(() => { const a = document.activeElement; return a === document.body ? "body" : a.closest("#keys-help") ? "dialog" : "page:" + a.tagName; })()`)); }
check("Tab never reaches the page behind it", visited.includes("dialog") && !visited.some(v => v.startsWith("page:")), visited.join(","));
await press("Escape"); await sleep(200);
check("Esc closes it", await js(`!document.getElementById("keys-help").open`), "still open");

console.log("--- switched off");
await js(`document.getElementById("keys-button").click()`); await sleep(200);
check("the footer opens the help without any shortcut", await js(`document.getElementById("keys-help").open`), "not open");
await js(`document.getElementById("keys-enabled").click()`);
await press("Escape"); await sleep(200);
await press("j"); await settle();
check("off: j does nothing", await js("scrollY") === before, await js("scrollY"));
await press("?"); await sleep(200);
check("off: not even ?", await js(`!document.getElementById("keys-help").open`), "opened");
await press("/");
check("off: nor /", !(await active()).startsWith("#search-q"), await active());
await open("/test/01%20Formatting");
await press("j"); await settle();
check("off is remembered on the next page", await js("scrollY") === 0, await js("scrollY"));
await press("Tab");
check("Tab still works, and reaches the skip link first", (await active()) === "a.skip|Skip to content", await active());
await press("Enter"); await sleep(200);
check("which leads to the content", await js(`location.hash`) === "#content", await js("location.hash"));
await js(`document.getElementById("keys-button").click()`); await sleep(200);
await js(`document.getElementById("keys-enabled").click()`);
await press("Escape"); await sleep(200);
await press("j"); await settle();
check("switched on again, j scrolls", await js("scrollY") > 0, await js("scrollY"));

console.log("--- recent searches, by arrow keys");
await open("/test/-/search?q=zeppelin");
await open("/test/-/search?q=dirigible");
await open("/test/01%20Formatting");
await press("/");
await press("ArrowDown");
check("Arrow down moves into the recent searches", (await active()) === "a|dirigible", await active());
await press("ArrowDown");
check("and on", (await active()) === "a|zeppelin", await active());
await press("ArrowUp"); await press("ArrowUp");
check("Arrow up leads back to the field", (await active()).startsWith("#search-q"), await active());

ws.close();
console.log(failures ? `\n${failures} check(s) failed` : "\nall checks passed");
process.exit(failures ? 1 : 0);
