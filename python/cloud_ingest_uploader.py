#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import time
from pathlib import Path
from typing import Any

import requests


CMD_MARKER_RE = re.compile(r"^\[gimble\.cmd\.(start|end)\]\s+(.*)$")


def _unescape_shell(value: str) -> str:
    if not value:
        return value
    return value.replace("\\\\", "\\").replace("\\ ", " ")


def _parse_cmd_marker(text: str) -> dict[str, str] | None:
    m = CMD_MARKER_RE.match(text)
    if not m:
        return None
    kind = m.group(1)
    rest = m.group(2)
    cmd = ""
    if " cmd=" in rest:
        head, cmd = rest.split(" cmd=", 1)
    else:
        head = rest
    meta: dict[str, str] = {"cmd_boundary": kind}
    for token in head.split():
        if "=" not in token:
            continue
        k, v = token.split("=", 1)
        meta[k.strip()] = _unescape_shell(v.strip())
    if cmd:
        meta["cmd"] = _unescape_shell(cmd.strip())
    return meta


def _event_id(session_id: str, seq: int, text: str) -> str:
    h = hashlib.sha1(f"{session_id}:{seq}:{text}".encode("utf-8", errors="ignore")).hexdigest()[:20]
    return f"evt_{h}"


def _post(url: str, token: str, device_id: str, payload: dict[str, Any], timeout: float = 4.0) -> None:
    headers = {"Content-Type": "application/json"}
    if token:
        headers["X-Gimble-Token"] = token
    if device_id:
        headers["X-Gimble-Device"] = device_id
    r = requests.post(url, headers=headers, data=json.dumps(payload), timeout=timeout)
    if r.status_code in {401, 410}:
        body = r.text.strip()
        if len(body) > 200:
            body = body[:200] + "..."
        print(f"uploader stopped ({r.status_code}): {body or 'unauthorized/expired'}", file=sys.stderr)
        raise SystemExit(1)
    r.raise_for_status()


def tail_and_upload(*, log_path: str, ingest_url: str, token: str, session_id: str, user_id: str, device_id: str = "", source: str = "terminal") -> None:
    p = Path(log_path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.touch(exist_ok=True)

    seq = 0
    adapter_hint = os.getenv("GIMBLE_LOG_HINT", "").strip().lower()
    with p.open("r", encoding="utf-8", errors="ignore") as f:
        f.seek(0, os.SEEK_END)
        while True:
            line = f.readline()
            if not line:
                time.sleep(0.25)
                continue

            text = line.strip()
            if not text:
                continue

            seq += 1
            marker = _parse_cmd_marker(text)
            if marker:
                text = (
                    f"command {marker.get('cmd_boundary','event')}: {marker.get('cmd','').strip()} "
                    f"(cwd={marker.get('cwd','')}, status={marker.get('status','')})"
                ).strip()
            sev = "info"
            lower = text.lower()
            if "error" in lower or "traceback" in lower:
                sev = "error"
            elif "warn" in lower:
                sev = "warning"

            payload = {
                "event_id": _event_id(session_id, seq, text),
                "session_id": session_id,
                "user_id": user_id,
                "ts_unix_ms": int(time.time() * 1000),
                "sequence": seq,
                "source": source,
                "severity": sev,
                "text": text,
                "metadata": {},
            }
            if adapter_hint:
                payload["metadata"]["adapter_hint"] = adapter_hint
                payload["metadata"]["protocol"] = adapter_hint
            if marker:
                payload["metadata"].update(marker)
            try:
                _post(ingest_url, token, device_id, payload)
            except Exception:
                # Best-effort uploader: drop transient failures and continue.
                time.sleep(0.8)


def main() -> int:
    ap = argparse.ArgumentParser(description="Gimble cloud event uploader")
    ap.add_argument("--log-path", required=True)
    ap.add_argument("--ingest-url", required=True)
    ap.add_argument("--token", default="")
    ap.add_argument("--device-id", default="")
    ap.add_argument("--session-id", required=True)
    ap.add_argument("--user-id", required=True)
    ap.add_argument("--source", default="terminal")
    args = ap.parse_args()

    tail_and_upload(
        log_path=args.log_path,
        ingest_url=args.ingest_url,
        token=args.token,
        session_id=args.session_id,
        user_id=args.user_id,
        device_id=args.device_id,
        source=args.source,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
