#!/usr/bin/env python3
"""Generalized live sanitizer for terminal session logs.

This parser is designed for noisy interactive shells (oh-my-zsh, powerlevel,
terminfo redraws, bracketed paste, cursor movement). It aims to produce clean,
readable, timestamped plain-text logs.
"""

from __future__ import annotations

import datetime as _dt
import os
import re
import sys
import time
from typing import Iterable


PROMPT_ONLY_RE = [
    re.compile(r"^[\s\W]*[#$%❯➜›»]\s*$"),
    re.compile(r"^[\w.-]+@[\w.-]+[: ].*[#$%]\s*$"),
]

BOX_CHARS = set("╭╮╰╯─│┌┐└┘├┤┬┴┼")
POWERLINE_CHARS = set("")
PROMPT_MARKERS = (" on ", "Py base", " at ")

TUI_HINT_LINE = "[tui] Full-screen output detected; provide clean snapshot or structured log."

TMUX_STATUS_RE = re.compile(r"^[\\w\\s\\-\\|\\[\\]:/\\.]+\\s+\\d+%$")
ROS_LINE_RE = re.compile(
    r"(\brosout\b|\brclcpp\b|\bros2\b|\bROS_(INFO|WARN|ERROR|DEBUG)\b|\[[A-Z]+\]\s*\[[0-9.]+\]\s*\[[^\]]+\])",
    re.IGNORECASE,
)
CMD_MARKER_RE = re.compile(r"^\[gimble\.cmd\.(start|end)\]")


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _env_str(name: str, default: str) -> str:
    raw = os.getenv(name, "").strip()
    return raw if raw else default


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _env_bool(name: str, default: bool) -> bool:
    raw = os.getenv(name, "").strip().lower()
    if not raw:
        return default
    return raw in {"1", "true", "yes", "on"}


ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]")


def _strip_ansi_text(text: str) -> str:
    if not text:
        return ""
    text = ANSI_ESCAPE_RE.sub("", text)
    text = text.replace("\x1b", "")
    return text


def _alnum_ratio(text: str) -> float:
    if not text:
        return 0.0
    total = len(text)
    if total == 0:
        return 0.0
    alnum = sum(ch.isalnum() for ch in text)
    return alnum / total


def _terminal_strict_mode() -> bool:
    if os.getenv("GIMBLE_TERMINAL_STRICT", "").strip().lower() in {"1", "true", "yes"}:
        return True
    return any(
        os.getenv(k)
        for k in (
            "TMUX",
            "TERMINATOR_UUID",
            "VTE_VERSION",
            "KONSOLE_VERSION",
            "TERM_PROGRAM",
            "TERMINAL_EMULATOR",
        )
    )


def _looks_like_ros_line(line: str) -> bool:
    if ROS_LINE_RE.search(line):
        return True
    if "/rosout" in line:
        return True
    if re.search(r"/[A-Za-z0-9_]+(/[A-Za-z0-9_]+)+", line):
        return True
    if re.search(r"[A-Za-z0-9_]+/[A-Za-z0-9_]+", line):
        return True
    return False


def _signal_line(line: str, min_ratio: float) -> bool:
    s = line.strip()
    if not s:
        return False
    if s.startswith("[tui:"):
        s = re.sub(r"^\[tui:[^\]]+\]\s*", "", s)
    elif s.startswith("[tui]"):
        s = re.sub(r"^\[tui\]\s*", "", s)
    s = _strip_ansi_text(s).strip()
    if not s:
        return False
    ratio = _alnum_ratio(s)
    return ratio >= min_ratio


class ANSIStreamStripper:
    NORMAL = 0
    ESC = 1
    CSI = 2
    OSC = 3
    DCS = 4
    ST_ESC = 5

    def __init__(self) -> None:
        self.state = self.NORMAL

    def feed(self, data: bytes) -> str:
        out = bytearray()
        i = 0
        n = len(data)
        while i < n:
            b = data[i]

            if self.state == self.NORMAL:
                if b == 0x1B:
                    self.state = self.ESC
                elif 0x80 <= b <= 0x9F:
                    pass
                elif b in (0x00, 0x07, 0x0B, 0x0C):
                    pass
                else:
                    out.append(b)
                i += 1
                continue

            if self.state == self.ESC:
                if b == ord("["):
                    self.state = self.CSI
                elif b == ord("]"):
                    self.state = self.OSC
                elif b in (ord("P"), ord("_"), ord("^"), ord("X")):
                    self.state = self.DCS
                else:
                    self.state = self.NORMAL
                i += 1
                continue

            if self.state == self.CSI:
                if 0x40 <= b <= 0x7E:
                    self.state = self.NORMAL
                i += 1
                continue

            if self.state in (self.OSC, self.DCS):
                if b == 0x07:
                    self.state = self.NORMAL
                    i += 1
                    continue
                if b == 0x1B:
                    self.state = self.ST_ESC
                    i += 1
                    continue
                i += 1
                continue

            if self.state == self.ST_ESC:
                if b == ord("\\"):
                    self.state = self.NORMAL
                else:
                    self.state = self.OSC
                i += 1
                continue

        text = out.decode("utf-8", errors="ignore")
        # caret-encoded escapes sometimes appear in recorded streams
        text = re.sub(r"\^\[[0-?]*[ -/]*[@-~]", "", text)
        text = text.replace("^M", "")
        return text


class LineReconstructor:
    def __init__(self) -> None:
        self.buf: list[str] = []
        self.cursor = 0

    def _line(self) -> str:
        return "".join(self.buf)

    def feed(self, text: str) -> Iterable[str]:
        for ch in text:
            if ch == "\r":
                self.cursor = 0
                continue
            if ch == "\b":
                if self.cursor > 0:
                    self.cursor -= 1
                    del self.buf[self.cursor]
                continue
            if ch == "\n":
                line = self._line()
                self.buf.clear()
                self.cursor = 0
                yield line
                continue

            o = ord(ch)
            if ch != "\t" and (o < 32 or o == 127):
                continue

            if self.cursor >= len(self.buf):
                self.buf.append(ch)
            else:
                self.buf[self.cursor] = ch
            self.cursor += 1


class TUIScreenBuffer:
    def __init__(
        self,
        cols: int = 120,
        rows: int = 40,
        mode: str = "reconstruct",
        interval: float = 2.0,
        max_lines: int = 60,
        min_alnum_ratio: float = 0.2,
        strip_box: bool = True,
        pseudo_ttl: float = 4.0,
    ) -> None:
        self.cols = max(40, cols)
        self.rows = max(15, rows)
        self.mode = mode
        self.interval = max(0.2, interval)
        self.max_lines = max(10, max_lines)
        self.min_alnum_ratio = max(0.02, min_alnum_ratio)
        self.strip_box = strip_box
        self.pseudo_ttl = max(0.5, pseudo_ttl)
        self.alt = False
        self.active = False
        self.active_until = 0.0
        self.row = 0
        self.col = 0
        self._screen = [[" "] * self.cols for _ in range(self.rows)]
        self.last_emit = 0.0
        self.last_activity = time.time()
        self.last_hint = 0.0
        self.dirty = False

    def _clear_screen(self) -> None:
        self._screen = [[" "] * self.cols for _ in range(self.rows)]
        self.row = 0
        self.col = 0
        self.dirty = True

    def _activate_pseudo(self) -> None:
        now = time.time()
        self.active = True
        self.active_until = max(self.active_until, now + self.pseudo_ttl)
        self.last_activity = now
        self.dirty = True

    def _clamp_pos(self) -> None:
        self.row = max(0, min(self.rows - 1, self.row))
        self.col = max(0, min(self.cols - 1, self.col))

    def _scroll(self) -> None:
        self._screen.pop(0)
        self._screen.append([" "] * self.cols)
        self.row = self.rows - 1
        self.col = 0

    def _put_char(self, ch: str) -> None:
        if self.row >= self.rows:
            self._scroll()
        if self.col >= self.cols:
            self.row += 1
            self.col = 0
            if self.row >= self.rows:
                self._scroll()
        self._screen[self.row][self.col] = ch
        self.col += 1
        self.dirty = True
        self.last_activity = time.time()

    def _handle_csi(self, params: str, final: str) -> None:
        if params.startswith("?") and final in {"h", "l"}:
            code = params[1:]
            if code in {"1049", "47", "1047"}:
                self.alt = final == "h"
                self.last_activity = time.time()
                self.dirty = True
                if self.alt:
                    self._clear_screen()
                return

        if not self.alt and final in {"H", "f", "A", "B", "C", "D", "J", "K"}:
            # Pseudo-TUI mode for full-screen apps that don't use alt screen.
            self._activate_pseudo()

        if not (self.alt or self.active):
            return

        nums = []
        for part in params.split(";"):
            if not part:
                continue
            if part.isdigit():
                nums.append(int(part))
            else:
                nums.append(None)
        n1 = nums[0] if len(nums) >= 1 and nums[0] is not None else 1
        n2 = nums[1] if len(nums) >= 2 and nums[1] is not None else 1

        if final in {"H", "f"}:
            self.row = max(0, n1 - 1)
            self.col = max(0, n2 - 1)
            self._clamp_pos()
            return
        if final == "A":
            self.row = max(0, self.row - n1)
            return
        if final == "B":
            self.row = min(self.rows - 1, self.row + n1)
            return
        if final == "C":
            self.col = min(self.cols - 1, self.col + n1)
            return
        if final == "D":
            self.col = max(0, self.col - n1)
            return
        if final == "J":
            if n1 in (2, 3):
                self._clear_screen()
                return
            if n1 == 0:
                for r in range(self.row, self.rows):
                    start = self.col if r == self.row else 0
                    for c in range(start, self.cols):
                        self._screen[r][c] = " "
                self.dirty = True
                return
        if final == "K":
            if n1 == 2:
                for c in range(self.cols):
                    self._screen[self.row][c] = " "
                self.dirty = True
                return
            if n1 == 0:
                for c in range(self.col, self.cols):
                    self._screen[self.row][c] = " "
                self.dirty = True
                return

    def _handle_escape(self, data: bytes, i: int) -> tuple[bool, int]:
        if i + 1 >= len(data):
            return True, 1
        if data[i + 1] != ord("["):
            return True, 2
        j = i + 2
        while j < len(data) and not (0x40 <= data[j] <= 0x7E):
            j += 1
        if j >= len(data):
            return True, len(data) - i
        params = data[i + 2 : j].decode("ascii", errors="ignore")
        final = chr(data[j])
        self._handle_csi(params, final)
        return True, j - i + 1

    def _snapshot_lines(self) -> list[str]:
        lines = ["".join(row).rstrip() for row in self._screen]
        candidates: list[tuple[float, int, str]] = []
        for idx, line in enumerate(lines):
            raw = line.rstrip()
            if not raw.strip():
                continue
            raw = _strip_ansi_text(raw)
            if not raw.strip():
                continue
            if self.strip_box:
                cleaned = "".join(ch for ch in raw if ch not in BOX_CHARS)
            else:
                cleaned = raw
            cleaned = cleaned.strip()
            if not cleaned:
                continue
            ratio = _alnum_ratio(cleaned)
            if ratio < self.min_alnum_ratio:
                continue
            candidates.append((ratio, idx, cleaned))

        if not candidates:
            return []
        if len(candidates) > self.max_lines:
            top = sorted(candidates, key=lambda item: item[0], reverse=True)[: self.max_lines]
            top.sort(key=lambda item: item[1])
            return [item[2] for item in top]
        candidates.sort(key=lambda item: item[1])
        return [item[2] for item in candidates]

    def _label(self, lines: list[str]) -> str:
        joined = "\n".join(lines).lower()
        if "htop" in joined:
            return "htop"
        if re.search(r"\btop\b", joined):
            return "top"
        return "tui"

    def _maybe_emit(self) -> list[str]:
        if not (self.alt or self.active):
            return []
        now = time.time()
        if self.mode == "drop":
            if now - self.last_hint > 3:
                self.last_hint = now
                return [TUI_HINT_LINE]
            return []
        if (now - self.last_emit) < self.interval:
            return []
        lines = self._snapshot_lines()
        if lines:
            label = self._label(lines)
            self.last_emit = now
            self.dirty = False
            return [f"[tui:{label}] {line}" for line in lines if line.strip()]
        if (now - self.last_activity) > 3 and (now - self.last_hint) > 10:
            self.last_hint = now
            return [TUI_HINT_LINE]
        return []

    def feed(self, data: bytes) -> tuple[list[str], bytes]:
        out_plain = bytearray()
        i = 0
        while i < len(data):
            if self.active and time.time() > self.active_until:
                self.active = False
            b = data[i]
            if b == 0x1B:
                handled, consumed = self._handle_escape(data, i)
                if handled:
                    i += max(1, consumed)
                    continue
            if self.alt or self.active:
                if b == 0x0A:  # \n
                    self.row += 1
                    self.col = 0
                    self.last_activity = time.time()
                elif b == 0x0D:  # \r
                    self.col = 0
                elif b == 0x08:  # backspace
                    self.col = max(0, self.col - 1)
                elif b == 0x09:  # tab
                    self.col = min(self.cols - 1, ((self.col // 4) + 1) * 4)
                elif 32 <= b <= 126:
                    self._put_char(chr(b))
                i += 1
                continue
            out_plain.append(b)
            i += 1
        return self._maybe_emit(), bytes(out_plain)


class SessionNormalizer:
    def __init__(self) -> None:
        self.last_cmd: str | None = None

    def normalize(self, line: str) -> str:
        s = line.strip()
        s = re.sub(r"^\^D+", "", s).strip()
        if not s:
            return ""

        # Heuristic: if command got glued to immediate output, split by prior cmd.
        if self.last_cmd and s.startswith(self.last_cmd) and len(s) > len(self.last_cmd):
            tail = s[len(self.last_cmd) :]
            if tail and not tail.startswith((" ", "\t")):
                s = tail.lstrip()

        # Track likely command lines for next-line deglueing.
        if self._looks_like_command(s):
            self.last_cmd = s

        return s

    @staticmethod
    def _looks_like_command(s: str) -> bool:
        if len(s) > 200:
            return False
        if s.startswith(("/", "[")):
            return False
        if "  " in s:
            return False
        # Accept common shell command-like starts.
        return bool(re.match(r"^[A-Za-z0-9_.-]+([ \t].*)?$", s))


def looks_like_prompt_noise(line: str) -> bool:
    s = line.strip()
    if not s:
        return True
    if CMD_MARKER_RE.match(s):
        return False

    if _looks_like_ros_line(s):
        return False

    for pat in PROMPT_ONLY_RE:
        if pat.match(s):
            return True

    if s.startswith(("╭", "╰")):
        return True

    if any(marker in s for marker in PROMPT_MARKERS):
        return True

    if sum(ch in BOX_CHARS for ch in s) > max(2, len(s) // 8):
        return True

    if len(s) > 80 and (" on " in s or " at " in s):
        return True

    if s.startswith("..") and "/" in s:
        return True
    if s.endswith("%") and "/" in s:
        return True

    if _terminal_strict_mode():
        if TMUX_STATUS_RE.match(s):
            return True
        if any(ch in POWERLINE_CHARS for ch in s):
            return True
        if sum(ch in BOX_CHARS for ch in s) > max(1, len(s) // 10):
            return True

    return False


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: session_log_sanitizer.py <raw_log> <clean_log>", file=sys.stderr)
        return 2

    raw_path, clean_path = sys.argv[1], sys.argv[2]

    stripper = ANSIStreamStripper()
    recon = LineReconstructor()
    norm = SessionNormalizer()

    tui_mode = _env_str("GIMBLE_TUI_MODE", "reconstruct").lower()
    if tui_mode not in {"reconstruct", "drop"}:
        tui_mode = "reconstruct"
    tui_interval = _env_float("GIMBLE_TUI_SNAPSHOT_INTERVAL_SECS", 2.0)
    tui_max_lines = _env_int("GIMBLE_TUI_MAX_LINES", 60)
    tui_min_ratio = _env_float("GIMBLE_TUI_MIN_ALNUM_RATIO", 0.20)
    tui_strip_box = _env_bool("GIMBLE_TUI_STRIP_BOX", True)
    tui_pseudo_ttl = _env_float("GIMBLE_TUI_PSEUDO_TTL_SECS", 4.0)
    tui = TUIScreenBuffer(
        mode=tui_mode,
        interval=tui_interval,
        max_lines=tui_max_lines,
        min_alnum_ratio=tui_min_ratio,
        strip_box=tui_strip_box,
        pseudo_ttl=tui_pseudo_ttl,
    )

    with open(raw_path, "rb") as src, open(clean_path, "a", encoding="utf-8") as dst:
        src.seek(0)
        while True:
            chunk = src.read(8192)
            if not chunk:
                time.sleep(0.08)
                continue

            tui_lines, passthrough = tui.feed(chunk)
            for line in tui_lines:
                ts = _dt.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                dst.write(f"[{ts}] {line}\n")
                dst.flush()

            if passthrough:
                clean_chunk = stripper.feed(passthrough)
                for line in recon.feed(clean_chunk):
                    line = norm.normalize(line)
                    if not line:
                        continue
                    if not line.startswith("[tui") and not _signal_line(line, tui_min_ratio) and looks_like_prompt_noise(line):
                        continue
                    ts = _dt.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                    dst.write(f"[{ts}] {line}\n")
                    dst.flush()


if __name__ == "__main__":
    raise SystemExit(main())
