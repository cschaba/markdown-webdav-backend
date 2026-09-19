// Keyboard support, in the manner of Vim and of Vimium.
//
// What it must not break, because a keyboard is also how assistive technology
// drives a page:
//  - "Focus" is the browser's own focus, never a painted highlight, so a screen
//    reader says what is selected and the focus ring shows it.
//  - Single-character shortcuts can be switched off (WCAG 2.1, 2.1.4): they fire
//    by accident for people who dictate, and may clash with a screen reader's
//    own keys. The switch is in the help, which a link in the footer opens, so
//    it is reachable without any shortcut. The choice is remembered.
//  - Nothing fires while a field is being typed in, nor together with Ctrl,
//    Alt or Meta: those belong to the browser and the operating system.
//
// The help is made from the same list that defines the keys (BINDINGS), so it
// cannot list a key that does nothing, or miss one that does.
(function () {
  "use strict";

  var reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
  var article = document.querySelector("article");
  var dialog = document.getElementById("keys-help");
  function helpOpen() { return !!dialog && dialog.open; }
  // A page that is a list - a folder, search results, tags - is walked item by
  // item; a note is scrolled.
  var listItems = function () {
    return Array.prototype.slice.call(document.querySelectorAll("main > ul.list > li > a:first-of-type"));
  };
  var isList = function () { return !article && !helpOpen() && listItems().length > 0; };

  // ---- actions ----------------------------------------------------------

  // The open help is what the scrolling keys scroll: it is longer than a small
  // window, and the page behind a modal dialog is not to be moved.
  function scrolled() { return helpOpen() ? dialog : window; }
  function pageHeight() { return helpOpen() ? dialog.clientHeight : innerHeight; }
  function scrollByAmount(amount, repeated) {
    // A held key repeats faster than a smooth scroll finishes.
    scrolled().scrollBy({ top: amount, behavior: repeated || reducedMotion.matches ? "auto" : "smooth" });
  }
  function scrollToEnd(bottom) {
    scrolled().scrollTo({ top: bottom ? (helpOpen() ? dialog : document.documentElement).scrollHeight : 0 });
  }

  function moveInList(step) {
    var items = listItems(), at = items.indexOf(document.activeElement);
    var next = at < 0 ? (step > 0 ? 0 : items.length - 1) : Math.min(Math.max(at + step, 0), items.length - 1);
    focusItem(items[next]);
  }
  function focusItem(el) {
    if (!el) return;
    el.focus({ preventScroll: true });
    el.scrollIntoView({ block: "nearest" });
  }

  function headings() {
    return Array.prototype.slice.call((article || document).querySelectorAll("h1, h2, h3, h4, h5, h6"))
      .filter(function (h) { return !h.classList.contains("title") && h.getClientRects().length > 0; });
  }
  // A heading is focused through its <summary> where it has one: that is what
  // is focusable anyway, and Enter or Space on it folds the section.
  function focusTarget(h) { return h.closest("summary") || h; }
  function moveToHeading(step) {
    var hs = headings();
    if (!hs.length) return;
    var at = hs.map(focusTarget).indexOf(document.activeElement);
    if (at < 0) {
      // from where the reader is: the first heading below the top edge, or the last above it
      at = hs.findIndex(function (h) { return h.getBoundingClientRect().top > 8; });
      at = step > 0 ? (at < 0 ? hs.length : at) - 1 : (at < 0 ? hs.length : at);
    }
    var target = focusTarget(hs[Math.min(Math.max(at + step, 0), hs.length - 1)]);
    if (!target.hasAttribute("tabindex") && target.tagName !== "SUMMARY") target.setAttribute("tabindex", "-1");
    target.focus({ preventScroll: true });
    target.scrollIntoView({ block: "start", behavior: reducedMotion.matches ? "auto" : "smooth" });
  }

  function currentFold() {
    var active = document.activeElement && document.activeElement.closest("details.fold");
    if (active) return active;
    // otherwise the section the top of the window is in
    var folds = Array.prototype.slice.call(document.querySelectorAll("article details.fold"));
    var current = null;
    folds.forEach(function (d) { if (d.getBoundingClientRect().top <= 80) current = d; });
    return current || folds[0] || null;
  }
  function toggleFold() { var d = currentFold(); if (d) d.open = !d.open; }
  function setAllFolds(open) {
    var had = document.activeElement && document.activeElement.closest("article details.fold");
    document.querySelectorAll("article details.fold").forEach(function (d) { d.open = open; });
    // Folding hides the heading that had the focus, if it was inside another
    // section, and the focus would fall back to the page. It goes to the
    // outermost section instead: that is what is left to see of the place.
    if (had && !open) {
      for (var up = had.parentElement.closest("details.fold"); up; up = up.parentElement.closest("details.fold")) had = up;
      had.querySelector("summary").focus({ preventScroll: true });
      had.scrollIntoView({ block: "nearest" });
    }
  }

  function go(selector) {
    var link = document.querySelector(selector);
    if (link && link.href) location.href = link.href;
  }
  function goUp() {
    var crumbs = document.querySelectorAll("header nav.crumbs a");
    if (crumbs.length > 1) location.href = crumbs[crumbs.length - 2].href;
  }

  function focusSearch() {
    var field = document.getElementById("search-q");
    if (field) { field.focus(); field.select(); }
  }

  // ---- link hints ---------------------------------------------------------
  // f puts a letter or two on every link in view; typing them follows it.

  var hints = null; // {layer, items: [{label, el, tag}], typed, newTab}
  var HINT_LETTERS = "asdfghjklqwertzuiopyxcvbnm";

  function hintLabels(count) {
    var letters = HINT_LETTERS.split(""), labels = [];
    if (count <= letters.length) return letters.slice(0, count);
    // two letters each, so that no label is the beginning of another
    for (var i = 0; i < letters.length && labels.length < count; i++) {
      for (var j = 0; j < letters.length && labels.length < count; j++) labels.push(letters[i] + letters[j]);
    }
    return labels;
  }

  function startHints(newTab) {
    var targets = Array.prototype.slice.call(document.querySelectorAll("a[href], summary, button:not([hidden]), input:not([type=hidden]), select"))
      .filter(function (el) {
        if (el.closest("dialog, .search-history")) return false;
        var r = el.getBoundingClientRect();
        return r.width > 0 && r.height > 0 && r.bottom > 0 && r.top < innerHeight && r.right > 0 && r.left < innerWidth;
      });
    if (!targets.length) return;
    var layer = document.createElement("div");
    layer.className = "key-hints";
    layer.setAttribute("aria-hidden", "true"); // an overlay for the eye; the links themselves are unchanged
    var labels = hintLabels(targets.length);
    var items = targets.slice(0, labels.length).map(function (el, i) {
      var r = el.getBoundingClientRect(), tag = document.createElement("span");
      tag.textContent = labels[i];
      if (el.href) tag.dataset.href = el.getAttribute("href");
      // beside the link rather than on it, so that the link can still be read
      tag.style.left = Math.max(r.left - 8 - 9 * labels[i].length, 0) + "px";
      tag.style.top = Math.max(r.top + (r.height - 18) / 2, 0) + "px";
      layer.append(tag);
      return { label: labels[i], el: el, tag: tag };
    });
    document.body.append(layer);
    hints = { layer: layer, items: items, typed: "", newTab: newTab };
    addEventListener("scroll", stopHints, { once: true, passive: true });
  }

  function stopHints() {
    if (!hints) return;
    hints.layer.remove();
    hints = null;
  }

  function typeHint(key) {
    if (key === "Escape") return stopHints();
    if (key === "Backspace") hints.typed = hints.typed.slice(0, -1);
    else if (key.length === 1) hints.typed += key.toLowerCase();
    else return;
    var left = hints.items.filter(function (it) {
      var on = it.label.indexOf(hints.typed) === 0;
      it.tag.hidden = !on;
      return on;
    });
    if (left.length === 0) return stopHints();
    if (left.length === 1 && left[0].label === hints.typed) {
      var el = left[0].el, newTab = hints.newTab;
      stopHints();
      if (el.tagName === "A" && newTab) open(el.href, "_blank", "noopener");
      else if (el.tagName === "A" || el.tagName === "SUMMARY" || el.tagName === "BUTTON") el.click();
      else el.focus();
    }
  }

  // ---- the keys -----------------------------------------------------------
  // inHelp: works while the help is open too, where it scrolls the help.

  var BINDINGS = [
    { group: "Everywhere" },
    { keys: "/", does: "Focus the search field", run: focusSearch },
    { keys: "?", does: "Show this help", run: function () { showHelp(); } },
    { keys: "Esc", does: "Leave a field, close the help, cancel a key sequence" },
    { keys: "f", does: "Follow a link: letters appear on the links in view, type them", run: function () { startHints(false); } },
    { keys: "F", does: "The same, opening the link in a new tab", run: function () { startHints(true); } },
    { keys: "H", does: "Back in the browser's history", run: function () { history.back(); } },
    { keys: "L", does: "Forward in the browser's history", run: function () { history.forward(); } },
    { keys: "-", does: "Up, to the folder this page is in", run: goUp },
    { keys: "gh", does: "Go home, to the top of the vault", run: function () { go("header nav.crumbs a"); } },
    { keys: "gt", does: "Go to the tags", run: function () { go('header nav a[href$="/-/tags"]'); } },
    { keys: "gr", does: "Go to the graph", run: function () { go('header nav a[href$="/-/graph"]'); } },
    { keys: "gp", does: "Go to the PDF export of this page", run: function () { go('footer a[href*="/-/export"]'); } },
    { keys: "gs", does: "Go to the slide show of this note, if it has slides", run: function () { go('footer a[href*="/-/slides"]'); } },
    { keys: "Tab", does: "The browser's own way from link to link; Shift+Tab goes back" },

    { group: "Scrolling" },
    { keys: "j", alt: "k", inHelp: true, does: "Down and up; on a list, to the next and previous item", run: function (e) { isList() ? moveInList(1) : scrollByAmount(70, e.repeat); }, runAlt: function (e) { isList() ? moveInList(-1) : scrollByAmount(-70, e.repeat); } },
    { keys: "d", alt: "u", inHelp: true, does: "Half a page down and up", run: function (e) { scrollByAmount(pageHeight() / 2, e.repeat); }, runAlt: function (e) { scrollByAmount(-pageHeight() / 2, e.repeat); } },
    { keys: "gg", alt: "G", inHelp: true, does: "To the top and to the bottom; on a list, to the first and last item",
      run: function () { var i = listItems(); isList() ? focusItem(i[0]) : scrollToEnd(false); },
      runAlt: function () { var i = listItems(); isList() ? focusItem(i[i.length - 1]) : scrollToEnd(true); } },
    { keys: "Enter", does: "Open the item or link that has the focus" },

    { group: "In a note" },
    { keys: "]]", alt: "[[", does: "To the next and previous heading", run: function () { moveToHeading(1); }, runAlt: function () { moveToHeading(-1); } },
    { keys: "J", alt: "K", does: "The same, for keyboards where [ and ] are hard to reach", run: function () { moveToHeading(1); }, runAlt: function () { moveToHeading(-1); } },
    { keys: "za", does: "Fold or unfold the section at the heading that has the focus", run: toggleFold },
    { keys: "zM", alt: "zR", does: "Fold all sections, unfold all sections", run: function () { setAllFolds(false); }, runAlt: function () { setAllFolds(true); } },
  ];

  var sequences = {}, helpSequences = {}; // "gg" -> function
  BINDINGS.forEach(function (b) {
    [sequences].concat(b.inHelp ? [helpSequences] : []).forEach(function (s) {
      if (b.run) s[b.keys] = b.run;
      if (b.runAlt) s[b.alt] = b.runAlt;
    });
  });

  // ---- on and off -----------------------------------------------------------

  var OFF_KEY = "keys-off";
  function enabled() {
    try { return localStorage.getItem(OFF_KEY) !== "1"; } catch (e) { return true; }
  }
  function setEnabled(on) {
    try { on ? localStorage.removeItem(OFF_KEY) : localStorage.setItem(OFF_KEY, "1"); } catch (e) { /* not remembered, still applied */ }
    off = !on;
  }
  var off = !enabled();

  // ---- the help -------------------------------------------------------------

  function buildHelp() {
    var body = dialog.querySelector(".keys-table");
    if (body.childElementCount) return;
    var table = null;
    BINDINGS.forEach(function (b) {
      if (b.group) {
        var h = document.createElement("h3");
        h.textContent = b.group;
        table = document.createElement("table");
        body.append(h, table);
        return;
      }
      var row = table.insertRow(), keys = row.insertCell(), does = row.insertCell();
      [b.keys].concat(b.alt ? [b.alt] : []).forEach(function (k, i) {
        if (i) keys.append(" ");
        var kbd = document.createElement("kbd");
        kbd.textContent = k;
        keys.append(kbd);
      });
      does.textContent = b.does;
    });
  }
  function showHelp() {
    if (!dialog || dialog.open) return;
    buildHelp();
    dialog.querySelector("#keys-enabled").checked = !off;
    dialog.showModal(); // modal: the focus stays inside, Esc closes, the focus returns
  }
  if (dialog) {
    dialog.querySelector("#keys-enabled").addEventListener("change", function (e) { setEnabled(e.target.checked); });
    var opener = document.getElementById("keys-button");
    if (opener) { opener.hidden = false; opener.addEventListener("click", showHelp); }
  }

  // ---- reading the keyboard -------------------------------------------------

  var pending = "", pendingTimer = null;
  var badge = document.createElement("div");
  badge.className = "key-pending";
  badge.setAttribute("aria-hidden", "true");
  badge.hidden = true;
  document.body.append(badge);
  function setPending(value) {
    pending = value;
    clearTimeout(pendingTimer);
    if (value) (helpOpen() ? dialog : document.body).append(badge); // a modal dialog covers the page
    badge.hidden = !value;
    badge.textContent = value;
    if (value) pendingTimer = setTimeout(function () { setPending(""); }, 1500);
  }

  function typing(target) {
    return target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName);
  }

  addEventListener("keydown", function (e) {
    // Keys of the search field's list of recent searches. Not single-character
    // shortcuts, so they work whether or not those are switched on.
    var search = e.target.closest && e.target.closest("form.search");
    if (search && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
      var stops = [search.querySelector("input")].concat(Array.prototype.slice.call(search.querySelectorAll(".search-history a")));
      var next = stops[stops.indexOf(document.activeElement) + (e.key === "ArrowDown" ? 1 : -1)];
      if (next) { next.focus(); e.preventDefault(); }
      return;
    }
    if (e.key === "Escape") {
      if (hints) { stopHints(); e.preventDefault(); }
      else if (pending) setPending("");
      else if (search || typing(e.target)) document.activeElement.blur();
      return;
    }
    // AltGr arrives as Ctrl+Alt on some systems; it is how [ and ] are typed on many keyboards.
    var altGr = e.getModifierState && e.getModifierState("AltGraph");
    if (off || e.metaKey || ((e.ctrlKey || e.altKey) && !altGr) || e.isComposing) return;
    if (hints) { typeHint(e.key); e.preventDefault(); return; }
    // In the help only the scrolling keys act. A character that is no key of
    // ours is taken from the browser all the same, there and on the page:
    // Firefox would answer "/", "'" or, with "search for text when you start
    // typing", any letter with its own find bar - a slip of the finger next to
    // a shortcut, or the second key of a sequence that leads nowhere. Space
    // stays the browser's: it scrolls, and presses the button or checkbox that
    // has the focus. So does all of it once the shortcuts are switched off.
    // The help has no field to type in; its checkbox is not one.
    var inHelp = helpOpen(), known = inHelp ? helpSequences : sequences;
    if (e.key.length !== 1 || e.key === " " || (!inHelp && typing(e.target))) return;
    e.preventDefault();

    // "g" may become "gg" or "gt": wait for the next key. A sequence that
    // leads nowhere ("gx") is dropped, and its last key tried on its own.
    var attempts = pending ? [pending + e.key, e.key] : [e.key];
    setPending("");
    for (var i = 0; i < attempts.length; i++) {
      var tried = attempts[i];
      if (known[tried]) { known[tried](e); return; }
      if (Object.keys(known).some(function (s) { return s.indexOf(tried) === 0; })) { setPending(tried); return; }
    }
  });
})();
