// The graph view: notes as dots, links as lines, coloured by tag groups.
// Layout, zoom, pan and drag are the force-graph library's; this file decides
// what is shown and how it looks.
(function () {
  "use strict";

  var host = document.getElementById("graph");
  var status = document.getElementById("graph-status");
  if (!host) return;
  if (typeof ForceGraph === "undefined") {
    status.textContent = "The graph library could not be loaded (it comes from cdn.jsdelivr.net).";
    return;
  }

  // Group colours. A graph puts any two groups side by side, so every pair of
  // hues must be tellable apart, also with a colour vision deficiency. Of the
  // validated eight-hue palette only these four pass that test on both of this
  // site's backgrounds. More groups therefore reuse the hues with a second
  // shape instead of adding hues that look alike: groups 1-4 are circles,
  // 5-8 diamonds. Do not add a fifth colour without validating all pairs.
  var HUES = {
    light: ["#2a78d6", "#eda100", "#e87ba4", "#008300"],
    dark: ["#3987e5", "#c98500", "#d55181", "#008300"],
  };
  var MAX_GROUPS = HUES.light.length * 2;
  var MAX_FIT_ZOOM = 3.2;

  var FILTERS = { tags: false, attachments: false, existing: false, orphans: true };
  try {
    var saved = JSON.parse(localStorage.getItem("graph-filters") || "{}");
    Object.keys(FILTERS).forEach(function (k) { if (typeof saved[k] === "boolean") FILTERS[k] = saved[k]; });
  } catch (e) { /* private window: the defaults do */ }

  var all = { nodes: [], links: [] }; // everything the server knows
  var groups = [];                    // [{tag, count}], fixed for the page's lifetime
  var neighbours = new Map();         // node id -> Set of ids, within what is shown
  var hovered = null, isolated = null, query = "";
  var theme = readTheme();
  var graph;

  function readTheme() {
    var css = getComputedStyle(document.documentElement);
    var dark = matchMedia("(prefers-color-scheme: dark)").matches;
    return {
      hues: dark ? HUES.dark : HUES.light,
      fg: css.getPropertyValue("--fg").trim(),
      muted: css.getPropertyValue("--muted").trim(),
      bg: css.getPropertyValue("--bg").trim(),
      // The rule colour is too faint for a hairline on the dark background.
      link: translucent(css.getPropertyValue("--muted").trim(), dark ? 0.55 : 0.35),
    };
  }

  function translucent(hex, alpha) {
    var n = parseInt(hex.slice(1), 16);
    return "rgba(" + (n >> 16) + "," + ((n >> 8) & 255) + "," + (n & 255) + "," + alpha + ")";
  }

  function topTag(tag) { return tag.split("/")[0]; }

  // A note belongs to the first group, in legend order, that it is tagged with.
  function groupOf(node) {
    if (node.kind !== "note" || !node.tags) return -1;
    for (var g = 0; g < groups.length; g++) {
      for (var t = 0; t < node.tags.length; t++) {
        if (topTag(node.tags[t]) === groups[g].tag) return g;
      }
    }
    return -1;
  }

  function computeGroups() {
    var counts = new Map();
    all.nodes.forEach(function (n) {
      if (n.kind !== "note" || !n.tags) return;
      new Set(n.tags.map(topTag)).forEach(function (t) { counts.set(t, (counts.get(t) || 0) + 1); });
    });
    // From the whole vault, never from what a filter leaves: a filter must
    // not repaint the notes that stay.
    groups = Array.from(counts, function (e) { return { tag: e[0], count: e[1] }; })
      .sort(function (a, b) { return b.count - a.count || a.tag.localeCompare(b.tag); })
      .slice(0, MAX_GROUPS);
    all.nodes.forEach(function (n) { n.group = groupOf(n); });
  }

  function visible() {
    var keep = all.nodes.filter(function (n) {
      if (n.kind === "tag") return FILTERS.tags;
      if (n.kind === "attachment") return FILTERS.attachments;
      if (n.kind === "missing") return !FILTERS.existing;
      return true;
    });
    var ids = new Set(keep.map(function (n) { return n.id; }));
    // Fresh link objects: the library replaces source and target by nodes.
    var links = all.links.filter(function (l) { return ids.has(l.source) && ids.has(l.target); })
      .map(function (l) { return { source: l.source, target: l.target }; });

    neighbours = new Map();
    keep.forEach(function (n) { neighbours.set(n.id, new Set()); });
    links.forEach(function (l) { neighbours.get(l.source).add(l.target); neighbours.get(l.target).add(l.source); });
    if (!FILTERS.orphans) {
      keep = keep.filter(function (n) { return neighbours.get(n.id).size > 0; });
    }
    keep.forEach(function (n) { n.degree = neighbours.get(n.id).size; });
    return { nodes: keep, links: links };
  }

  function radius(node) { return 2.5 + Math.sqrt(node.degree || 0) * 1.6; }

  // Whether a node stands out while something is hovered, isolated or searched.
  function emphasised(node) {
    if (hovered) return node === hovered || neighbours.get(hovered.id).has(node.id);
    if (isolated !== null && node.group !== isolated) return false;
    if (query) return node.title.toLowerCase().indexOf(query) >= 0;
    return true;
  }
  function anyFocus() { return hovered || isolated !== null || query; }

  function drawNode(node, ctx, scale) {
    var r = radius(node), strong = emphasised(node);
    ctx.globalAlpha = strong ? 1 : 0.12;
    var colour = node.group >= 0 ? theme.hues[node.group % theme.hues.length] : theme.muted;
    ctx.beginPath();
    if (node.kind === "attachment") {
      ctx.rect(node.x - r * 0.8, node.y - r * 0.8, r * 1.6, r * 1.6);
    } else if (node.kind === "tag" || node.group >= theme.hues.length) {
      var d = r * 1.25; // a diamond of about the circle's area
      ctx.moveTo(node.x, node.y - d); ctx.lineTo(node.x + d, node.y);
      ctx.lineTo(node.x, node.y + d); ctx.lineTo(node.x - d, node.y); ctx.closePath();
    } else {
      ctx.arc(node.x, node.y, r, 0, 2 * Math.PI);
    }
    if (node.kind === "missing" || node.kind === "tag") { // hollow: not a note
      ctx.lineWidth = 1.2 / scale; ctx.strokeStyle = colour; ctx.stroke();
    } else {
      ctx.fillStyle = colour; ctx.fill();
      // A ring in the background colour keeps touching dots apart.
      ctx.lineWidth = 1 / scale; ctx.strokeStyle = theme.bg; ctx.stroke();
    }
    // Names appear once there is room for them, or for what is singled out.
    if ((scale > 1.8 && strong) || (anyFocus() && strong && (hovered || scale > 0.9))) {
      var size = 11 / scale;
      ctx.font = size + "px system-ui, sans-serif";
      ctx.textAlign = "center"; ctx.textBaseline = "top";
      ctx.fillStyle = theme.fg; // text wears the text colour, never the group's
      ctx.fillText(node.title, node.x, node.y + r + 2 / scale);
    }
    ctx.globalAlpha = 1;
  }

  function linkColour(link) {
    var on = !anyFocus() || (hovered ? (link.source === hovered || link.target === hovered)
      : emphasised(link.source) && emphasised(link.target));
    return on ? (hovered ? theme.muted : theme.link) : "transparent";
  }

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function tooltip(node) {
    var what = { attachment: "attachment", missing: "note that does not exist yet", tag: "tag" }[node.kind];
    if (!what) what = node.group >= 0 ? "group #" + groups[node.group].tag : "note";
    var n = node.degree || 0;
    return "<strong>" + escapeHTML(node.title) + "</strong><br>" + escapeHTML(what) + " · " + n + (n === 1 ? " link" : " links");
  }

  function refresh() {
    var data = visible();
    // A few notes get room for their names; hundreds must stay compact.
    graph.d3Force("link").distance(data.nodes.length < 80 ? 60 : 30);
    graph.graphData(data);
    document.getElementById("graph-count").textContent =
      data.nodes.length + " shown of " + all.nodes.length + " · " + data.links.length + " links";
  }

  function legend() {
    var list = document.getElementById("graph-groups");
    list.textContent = "";
    groups.forEach(function (g, i) {
      var item = document.createElement("li"), button = document.createElement("button");
      var mark = document.createElement("span");
      mark.className = "mark" + (i >= theme.hues.length ? " diamond" : "");
      mark.style.background = theme.hues[i % theme.hues.length];
      button.type = "button";
      button.setAttribute("aria-pressed", String(isolated === i));
      button.append(mark, "#" + g.tag + " ", Object.assign(document.createElement("small"), { textContent: g.count }));
      button.addEventListener("click", function () {
        isolated = isolated === i ? null : i;
        legend();
        graph.nodeColor(graph.nodeColor()); // repaint
      });
      item.append(button);
      list.append(item);
    });
    if (!groups.length) list.innerHTML = "<li class=\"hint\">No tags in this vault yet.</li>";
  }

  var userMoved = false;
  function fit() {
    if (userMoved) return;
    // Frame what is linked. Unlinked notes drift around it; framing them
    // too would shrink the actual graph to a blot in the middle.
    var linked = graph.graphData().nodes.some(function (n) { return n.degree > 0; });
    var box = graph.getGraphBbox(function (n) { return !linked || n.degree > 0; });
    if (!box) return;
    // Computed here rather than with the library's zoomToFit, for two things
    // it cannot do: leave out the strip the open panel covers, and stop
    // zooming in before a handful of notes fills the window.
    var canvas = document.getElementById("graph-canvas").getBoundingClientRect();
    var panel = document.getElementById("graph-panel");
    var covered = panel.open && canvas.width > 640 ? panel.getBoundingClientRect().width + 16 : 0;
    var pad = 40, width = canvas.width - covered;
    var zoom = Math.min(
      (width - 2 * pad) / Math.max(box.x[1] - box.x[0], 1),
      (canvas.height - 2 * pad) / Math.max(box.y[1] - box.y[0], 1),
      MAX_FIT_ZOOM);
    graph.centerAt((box.x[0] + box.x[1]) / 2 + covered / 2 / zoom, (box.y[0] + box.y[1]) / 2, 300);
    graph.zoom(zoom, 300);
  }

  function resize() {
    var box = document.getElementById("graph-canvas").getBoundingClientRect();
    graph.width(box.width).height(box.height);
  }

  fetch(host.dataset.source, { credentials: "same-origin" })
    .then(function (res) { if (!res.ok) throw new Error(res.status); return res.json(); })
    .then(function (data) {
      all = data;
      computeGroups();
      status.remove();
      graph = new ForceGraph(document.getElementById("graph-canvas"))
        .backgroundColor("rgba(0,0,0,0)")
        .nodeLabel(tooltip)
        .nodeCanvasObject(drawNode)
        .nodePointerAreaPaint(function (node, colour, ctx) {
          ctx.fillStyle = colour; ctx.beginPath(); // a target bigger than the dot
          ctx.arc(node.x, node.y, radius(node) + 3, 0, 2 * Math.PI); ctx.fill();
        })
        .linkColor(linkColour)
        .linkWidth(function (l) { return hovered && (l.source === hovered || l.target === hovered) ? 1.5 : 0.8; })
        .onNodeHover(function (node) {
          hovered = node || null;
          document.getElementById("graph-canvas").style.cursor = node && node.url ? "pointer" : "";
        })
        .onNodeClick(function (node) { if (node.url) location.href = node.url; })
        .cooldownTime(6000)
        // Keep the graph in view while it unfolds and once it rests...
        .onEngineStop(fit);
      // ...but never take the view away from someone who has moved it.
      ["pointerdown", "wheel", "touchstart"].forEach(function (type) {
        document.getElementById("graph-canvas").addEventListener(type, function () { userMoved = true; }, { passive: true });
      });
      // Repulsion without a range pushes unlinked notes ever further out.
      graph.d3Force("charge").distanceMax(450);
      // On a phone the open panel would cover the graph it controls.
      if (matchMedia("(max-width: 640px)").matches) document.getElementById("graph-panel").open = false;
      document.getElementById("graph-panel").addEventListener("toggle", fit);
      resize();
      legend();
      refresh();
      [400, 1200, 2500, 4000].forEach(function (ms) { setTimeout(fit, ms); });

      addEventListener("resize", resize);
      matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        theme = readTheme(); legend(); graph.nodeColor(graph.nodeColor());
      });
      document.querySelectorAll("[data-filter]").forEach(function (box) {
        box.checked = FILTERS[box.dataset.filter];
        box.addEventListener("change", function () {
          FILTERS[box.dataset.filter] = box.checked;
          userMoved = false; // what is shown changed: frame it again
          try { localStorage.setItem("graph-filters", JSON.stringify(FILTERS)); } catch (e) { /* not remembered, still applied */ }
          refresh();
        });
      });
      document.getElementById("graph-search").addEventListener("input", function (e) {
        query = e.target.value.trim().toLowerCase();
        graph.nodeColor(graph.nodeColor());
      });
    })
    .catch(function (err) { status.textContent = "The graph could not be loaded: " + err.message; });
})();
