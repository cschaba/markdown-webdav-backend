#!/usr/bin/env python3
"""Time and tokens of a Claude Code session, from its transcript.

    tools/session-stats.py ~/.claude/projects/<project>/<session>.jsonl

Claude Code writes one JSON object per line. Assistant entries carry the usage
the API reported for that response; a response that arrives in several entries
repeats it, so each response id is counted once. Time is split at the person's
prompts: from a prompt to the last action before the next one is the model
working, the rest is the person reading, trying and deciding.
"""
import collections
import datetime
import json
import sys


def when(row):
    stamp = row.get("timestamp")
    return datetime.datetime.fromisoformat(stamp.replace("Z", "+00:00")) if stamp else None


def is_prompt(row):
    """A message typed by the person, as opposed to a tool result or a notice."""
    if row.get("type") != "user" or row.get("isMeta") or row.get("isSidechain"):
        return False
    content = (row.get("message") or {}).get("content")
    if isinstance(content, list):
        if any(block.get("type") == "tool_result" for block in content):
            return False
        content = " ".join(block.get("text", "") for block in content if block.get("type") == "text")
    if not isinstance(content, str) or not content.strip():
        return False
    return not content.startswith(("<local-command", "<command-name", "<bash-")) and "task-notification" not in content[:300]


def clock(delta):
    return str(delta).split(".")[0]


def main(path):
    rows = [json.loads(line) for line in open(path) if line.strip()]

    usage, models = {}, collections.Counter()
    for row in rows:
        message = row.get("message") or {}
        if row.get("type") == "assistant" and message.get("usage") and message.get("id"):
            usage[message["id"]] = (message["usage"], message.get("model"))
    totals = collections.Counter()
    for used, model in usage.values():
        models[model] += 1
        for key in ("output_tokens", "input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"):
            totals[key] += used.get(key) or 0
    print(f"model responses: {len(usage)}  {dict(models)}")
    for key, value in totals.items():
        print(f"  {key:30} {value:>15,}")
    print(f"  {'total processed':30} {sum(totals.values()):>15,}")

    stamps = [when(row) for row in rows if when(row)]
    print(f"\nsession: {min(stamps).astimezone():%F %H:%M} -> {max(stamps).astimezone():%H:%M} = {clock(max(stamps) - min(stamps))}")
    prompts = [i for i, row in enumerate(rows) if is_prompt(row)]
    working = waiting = datetime.timedelta()
    turns = []
    for n, i in enumerate(prompts):
        j = prompts[n + 1] if n + 1 < len(prompts) else len(rows)
        span = [when(row) for row in rows[i:j] if when(row) and row.get("type") in ("assistant", "user")]
        if len(span) < 2:
            continue
        working += max(span) - span[0]
        turns.append(max(span) - span[0])
        if j < len(rows) and when(rows[j]):
            waiting += when(rows[j]) - max(span)
    print(f"prompts: {len(prompts)}")
    print(f"model working:      {clock(working)}")
    print(f"waiting for person: {clock(waiting)}")
    print(f"longest turns:      {[clock(t) for t in sorted(turns, reverse=True)[:5]]}")

    tools = collections.Counter()
    for row in rows:
        content = (row.get("message") or {}).get("content")
        if row.get("type") == "assistant" and isinstance(content, list):
            tools.update(block.get("name") for block in content if block.get("type") == "tool_use")
    print(f"tool calls: {sum(tools.values())}  {dict(tools.most_common(8))}")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(sys.argv[1])
