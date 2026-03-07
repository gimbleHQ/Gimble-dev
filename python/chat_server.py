#!/usr/bin/env python3
import argparse
import errno
import os
import platform
import re
import threading
import time
from pathlib import Path
from typing import Dict, List, Optional, Tuple

try:
    from flask import Flask, jsonify, request, send_from_directory
except ModuleNotFoundError as exc:
    missing = str(exc).split("'")[-2] if "'" in str(exc) else "flask"
    req_file = Path(__file__).resolve().parent / "requirements-core.txt"
    print(f"Missing Python package: {missing}")
    print("Install dependencies with:")
    print(f"  python3 -m pip install -r {req_file}")
    raise SystemExit(1)


DEFAULT_SYSTEM_PROMPT = """You are Gimble, the debugging and observability engine behind gimble.dev. Your task is to analyze terminal logs from a user's robot session. Logs may include ROS/ROS2 logs, system/OS logs, sensor logs (LiDAR, IMU, cameras), GPU/CPU/memory telemetry, inference logs, and other application processes. Use timestamps and log content to infer what the user recently did, what processes or ROS nodes are running, what topics or components appear active, and what the system is doing. Act as a robot debugging agent: interpret logs to extract meaning, identify errors, warnings, crashes, stalled nodes, abnormal resource usage, memory leaks, or other anomalies. Focus on signal over noise and produce concise, information-dense insights when analysis is requested. Logs will arrive incrementally in chunks over time (typically the last ~100 lines copied from the terminal). Continuously consume them and build context across messages. You may discard older normal context if memory becomes limited, but prioritize and retain any anomalies, crashes, warnings, faults, or abnormal behavior, and remain biased toward recent events. When the user sends logs, do not analyze or respond with a long message-simply acknowledge with "OK, received." When the user later asks questions such as what happened, what is going on, what recently occurred, why something failed, or what processes/nodes are active, then analyze the accumulated logs and provide clear debugging insights and explanations."""
REQ_FILE = Path(__file__).resolve().parent / "requirements-core.txt"

GROQ_MODELS = [
    "openai/gpt-oss-120b",
    "openai/gpt-oss-20b",
    "openai/gpt-oss-safeguard-20b",
    "qwen/qwen3-32b",
    "llama-3.1-8b-instant",
    "llama-3.3-70b-versatile",
]

OPENAI_MODELS = [
    "gpt-4o-mini",
    "gpt-4.1-mini",
    "gpt-4.1-nano",
]

DEFAULT_GROQ_MODEL = os.getenv("GROQ_MODEL", "openai/gpt-oss-120b")
DEFAULT_OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
DEFAULT_MODEL_KEY = f"groq:{DEFAULT_GROQ_MODEL}"

EXPERIMENTAL_GPTQ_KEY = "local:gptq-4k-experimental"
EXPERIMENTAL_GPTQ_LABEL = "GPT-Q 4K (Experimental, developer-only)"
LOG_INGEST_INTERVAL_SECONDS = 15.0
MAX_CONTEXT_CHARS = 24000
MAX_RECENT_LINES = 800
MAX_ANOMALY_LINES = 300
LOG_ACK_MESSAGE = "OK, received."

LOG_LIKE_LINE = re.compile(
    r"(^\[\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\])"
    r"|(^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2})"
    r"|(\b(INFO|WARN|WARNING|ERROR|FATAL|DEBUG|TRACE|EXCEPTION)\b)"
    r"|(\b(ros|ros2|node|topic|lcm|dds|telemetry|gpu|cpu|memory)\b)"
    ,
    re.IGNORECASE,
)
ANOMALY_LINE = re.compile(
    r"\b(error|warn|warning|fatal|exception|crash|failed|failure|timeout|oom|killed|segfault|fault|stalled|anomaly)\b",
    re.IGNORECASE,
)


def is_likely_log_dump(text: str) -> bool:
    lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
    if len(lines) < 5:
        return False
    scored = sum(1 for ln in lines if LOG_LIKE_LINE.search(ln))
    return scored >= 3


class TerminalContextStore:
    def __init__(self, log_path: Optional[Path], shell_pid: int) -> None:
        self.log_path = log_path
        self.shell_pid = shell_pid
        self._lock = threading.Lock()
        self._recent_lines: List[str] = []
        self._anomaly_lines: List[str] = []
        self._offset = 0
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._active = False
        self._last_ingest_at = 0.0

    def start(self) -> None:
        if not self.log_path:
            self._active = False
            return

        if self.log_path.exists():
            try:
                self._offset = self.log_path.stat().st_size
            except OSError:
                self._offset = 0

        self._active = self._shell_alive()
        if not self._active:
            return

        self._thread = threading.Thread(target=self._run, name="gimble-log-ingestor", daemon=True)
        self._thread.start()

    def is_active(self) -> bool:
        with self._lock:
            return self._active

    def status(self) -> Dict[str, object]:
        with self._lock:
            last_ingest_at = self._last_ingest_at
            active = self._active
        return {
            "active": active,
            "last_ingest_at": int(last_ingest_at) if last_ingest_at > 0 else None,
            "interval_seconds": int(LOG_INGEST_INTERVAL_SECONDS),
        }

    def ingest_text(self, text: str) -> None:
        if not text:
            return
        lines = [ln.rstrip() for ln in text.splitlines() if ln.strip()]
        if not lines:
            return
        with self._lock:
            self._recent_lines.extend(lines)
            if len(self._recent_lines) > MAX_RECENT_LINES:
                self._recent_lines = self._recent_lines[-MAX_RECENT_LINES:]

            for line in lines:
                if ANOMALY_LINE.search(line):
                    self._anomaly_lines.append(line)
            if len(self._anomaly_lines) > MAX_ANOMALY_LINES:
                self._anomaly_lines = self._anomaly_lines[-MAX_ANOMALY_LINES:]

            self._last_ingest_at = time.time()

    def render_context(self) -> str:
        with self._lock:
            if not self._active:
                return ""
            if not self._recent_lines and not self._anomaly_lines:
                return ""
            recent = "\n".join(self._recent_lines[-200:])
            anomalies = "\n".join(self._anomaly_lines[-80:])

        parts: List[str] = [
            "Live terminal context from current Gimble session (incremental background ingestion)."
        ]
        if anomalies:
            parts.append("Anomalies / warnings / failures (prioritized):")
            parts.append(anomalies)
        if recent:
            parts.append("Most recent terminal lines:")
            parts.append(recent)
        context = "\n\n".join(parts)
        if len(context) > MAX_CONTEXT_CHARS:
            context = context[-MAX_CONTEXT_CHARS:]
        return context

    def _shell_alive(self) -> bool:
        if self.shell_pid <= 0:
            return True
        try:
            os.kill(self.shell_pid, 0)
            return True
        except OSError as exc:
            if exc.errno == errno.ESRCH:
                return False
            return True

    def _read_new_bytes(self) -> str:
        if not self.log_path or not self.log_path.exists():
            return ""
        size = self.log_path.stat().st_size
        if size < self._offset:
            self._offset = 0
        if size == self._offset:
            return ""

        with self.log_path.open("r", encoding="utf-8", errors="replace") as f:
            f.seek(self._offset)
            chunk = f.read()
            self._offset = f.tell()
        return chunk

    def _run(self) -> None:
        while not self._stop.is_set():
            if not self._shell_alive():
                with self._lock:
                    self._active = False
                return
            chunk = self._read_new_bytes()
            if chunk:
                self.ingest_text(chunk)
            self._stop.wait(LOG_INGEST_INTERVAL_SECONDS)


def with_terminal_context(messages: List[Dict[str, str]], terminal_context: str) -> List[Dict[str, str]]:
    if not terminal_context:
        return messages

    context_message = {
        "role": "system",
        "content": (
            "Background terminal context is continuously ingested. Use it when the user asks for analysis. "
            "Do not proactively dump it unless asked.\n\n" + terminal_context
        ),
    }

    if not messages:
        return [context_message]
    if messages[0].get("role") == "system":
        return [messages[0], context_message] + messages[1:]
    return [context_message] + messages


def chat_env_path() -> Path:
    if os.name == "nt":
        base = Path(os.environ.get("APPDATA", Path.home()))
        return base / "gimble" / "chat.env"
    if platform.system().lower() == "darwin":
        return Path.home() / "Library" / "Application Support" / "gimble" / "chat.env"
    return Path.home() / ".config" / "gimble" / "chat.env"


def load_chat_env() -> Dict[str, str]:
    path = chat_env_path()
    if not path.exists():
        return {}
    values: Dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].strip()
        if "=" not in line:
            continue
        key, val = line.split("=", 1)
        values[key.strip()] = val.strip().strip('"').strip("'")
    return values


def split_system_prefix(text: str) -> Tuple[str, str]:
    stripped = text.strip()
    if not stripped.lower().startswith("system:"):
        return "", text

    first_line, _, tail = stripped.partition("\n")
    system_prompt = first_line[len("System:") :].strip()
    user_text = tail.strip()
    if user_text.lower().startswith("user:"):
        user_text = user_text[len("User:") :].strip()
    return system_prompt, user_text



class ConversationStore:
    def __init__(self, model_keys: List[str]) -> None:
        self._lock = threading.Lock()
        self._messages: Dict[str, List[Dict[str, str]]] = {
            key: [{"role": "system", "content": DEFAULT_SYSTEM_PROMPT}] for key in model_keys
        }

    def set_system_prompt(self, model_key: str, prompt: str) -> None:
        prompt = prompt.strip()
        if not prompt:
            return
        with self._lock:
            current = self._messages[model_key]
            remainder = [m for m in current[1:] if m.get("role") != "system"]
            self._messages[model_key] = [{"role": "system", "content": prompt}] + remainder
            self._trim(model_key)

    def append_user(self, model_key: str, text: str) -> List[Dict[str, str]]:
        with self._lock:
            self._messages[model_key].append({"role": "user", "content": text})
            self._trim(model_key)
            return list(self._messages[model_key])

    def append_assistant(self, model_key: str, text: str) -> None:
        with self._lock:
            self._messages[model_key].append({"role": "assistant", "content": text})
            self._trim(model_key)

    def _trim(self, model_key: str) -> None:
        msgs = self._messages[model_key]
        if len(msgs) > 31:
            self._messages[model_key] = [msgs[0]] + msgs[-30:]



class OpenAIBackend:
    def __init__(self) -> None:
        env = load_chat_env()
        self.api_key = os.getenv("OPENAI_API_KEY", env.get("OPENAI_API_KEY", "")).strip()
        self.default_model = os.getenv("OPENAI_MODEL", env.get("OPENAI_MODEL", DEFAULT_OPENAI_MODEL)).strip() or DEFAULT_OPENAI_MODEL

    def available(self) -> bool:
        return bool(self.api_key)

    def chat(self, messages: List[Dict[str, str]], model: str) -> str:
        if not self.api_key:
            raise RuntimeError("OPENAI_API_KEY is not configured. Set it in env or gimble chat.env")
        try:
            from openai import OpenAI
        except ModuleNotFoundError:
            raise RuntimeError(f"openai package is required. Run: python3 -m pip install -r {REQ_FILE}")

        client = OpenAI(api_key=self.api_key)
        target_model = model or self.default_model
        try:
            response = client.chat.completions.create(
                model=target_model,
                messages=messages,
                temperature=0.7,
            )
            text = response.choices[0].message.content or ""
            return text.strip() or "(empty response)"
        except Exception as exc:  # noqa: BLE001
            raise RuntimeError(f"OpenAI API error: {exc}")


class GroqBackend:
    def __init__(self) -> None:
        env = load_chat_env()
        self.api_key = os.getenv("GROQ_API_KEY", env.get("GROQ_API_KEY", "")).strip()
        self.default_model = os.getenv("GROQ_MODEL", env.get("GROQ_MODEL", DEFAULT_GROQ_MODEL)).strip() or DEFAULT_GROQ_MODEL

    def available(self) -> bool:
        return bool(self.api_key)

    def chat(self, messages: List[Dict[str, str]], model: str) -> str:
        if not self.api_key:
            raise RuntimeError("GROQ_API_KEY is not configured. Set it in env or gimble chat.env")
        try:
            from openai import OpenAI
        except ModuleNotFoundError:
            raise RuntimeError(f"openai package is required. Run: python3 -m pip install -r {REQ_FILE}")

        client = OpenAI(api_key=self.api_key, base_url="https://api.groq.com/openai/v1")
        target_model = model or self.default_model
        try:
            response = client.chat.completions.create(
                model=target_model,
                messages=messages,
                temperature=0.7,
            )
            text = response.choices[0].message.content or ""
            return text.strip() or "(empty response)"
        except Exception as exc:  # noqa: BLE001
            raise RuntimeError(f"Groq API error: {exc}")


def parse_model_key(model_key: str) -> Tuple[str, str]:
    provider, _, model = model_key.partition(":")
    if not provider or not model:
        return "", ""
    return provider, model


def create_app() -> Flask:
    app = Flask(__name__, static_folder=str(Path(__file__).resolve().parent / "web"), static_url_path="")

    openai_backend = OpenAIBackend()
    groq_backend = GroqBackend()

    model_options = []
    for model in GROQ_MODELS:
        model_options.append(
            {
                "key": f"groq:{model}",
                "label": model,
                "available": groq_backend.available(),
                "provider": "groq",
            }
        )
    for model in OPENAI_MODELS:
        model_options.append(
            {
                "key": f"openai:{model}",
                "label": model,
                "available": openai_backend.available(),
                "provider": "openai",
            }
        )

    model_options.append(
        {
            "key": EXPERIMENTAL_GPTQ_KEY,
            "label": EXPERIMENTAL_GPTQ_LABEL,
            "available": False,
            "provider": "local",
        }
    )

    valid_keys = [m["key"] for m in model_options]
    available_keys = [m["key"] for m in model_options if m["available"]]

    default_key = os.getenv("GIMBLE_DEFAULT_MODEL", DEFAULT_MODEL_KEY).strip()
    if default_key not in valid_keys:
        default_key = DEFAULT_MODEL_KEY
    if default_key not in available_keys and available_keys:
        default_key = available_keys[0]

    store = ConversationStore(valid_keys)

    raw_log_path = os.getenv("GIMBLE_SESSION_LOG_PATH", "").strip()
    session_log_path = Path(raw_log_path).expanduser() if raw_log_path else None
    try:
        session_shell_pid = int((os.getenv("GIMBLE_SESSION_SHELL_PID", "0") or "0").strip())
    except ValueError:
        session_shell_pid = 0

    terminal_context = TerminalContextStore(session_log_path, session_shell_pid)
    terminal_context.start()

    session_config = {
        "profile": os.getenv("GIMBLE_PROFILE", ""),
        "name": os.getenv("GIMBLE_USER_NAME", ""),
        "email": os.getenv("GIMBLE_USER_EMAIL", ""),
        "github": os.getenv("GIMBLE_USER_GITHUB", ""),
        "workspace_roots": [x.strip() for x in os.getenv("GIMBLE_WORKSPACE_ROOTS", "").split(",") if x.strip()],
        "ros_type": os.getenv("GIMBLE_ROS_TYPE", ""),
        "ros_distro": os.getenv("GIMBLE_ROS_DISTRO", ""),
        "ros_workspace": os.getenv("GIMBLE_ROS_WORKSPACE", ""),
        "obs_grafana_url": os.getenv("GIMBLE_OBS_GRAFANA_URL", ""),
        "obs_sentry_url": os.getenv("GIMBLE_OBS_SENTRY_URL", ""),
        "system_prompt_profile": os.getenv("GIMBLE_SYSTEM_PROMPT_PROFILE", ""),
        "notification_preference": os.getenv("GIMBLE_NOTIFICATION_PREFERENCE", ""),
    }

    @app.get("/")
    def index():
        return send_from_directory(app.static_folder, "index.html")

    @app.get("/api/session-config")
    def session_cfg():
        return jsonify(session_config)


    @app.get("/__gimble_proof")
    def gimble_proof():
        nonce = (request.args.get("nonce") or "").strip()
        if not nonce:
            return "missing nonce", 400
        return nonce, 200, {"Content-Type": "text/plain; charset=utf-8"}

    @app.get("/api/models")
    def models():
        return jsonify({"default": default_key, "models": model_options})

    @app.get("/api/context-status")
    def context_status():
        return jsonify(terminal_context.status())

    @app.post("/api/chat")
    def chat():
        payload = request.get_json(silent=True) or {}
        raw_text = (payload.get("message") or "").strip()
        model_key = (payload.get("model") or default_key).strip()
        explicit_system = (payload.get("system_prompt") or "").strip()

        if model_key not in valid_keys:
            return jsonify({"error": f"unknown model: {model_key}"}), 400

        provider, model_name = parse_model_key(model_key)

        prefixed_system, user_text = split_system_prefix(raw_text)
        system_prompt = explicit_system or prefixed_system
        if system_prompt:
            store.set_system_prompt(model_key, system_prompt)

        user_text = user_text.strip()
        if not user_text:
            if system_prompt:
                return jsonify({"reply": "System prompt updated for this model session."})
            return jsonify({"error": "message cannot be empty"}), 400

        history = store.append_user(model_key, user_text)

        if terminal_context.is_active() and is_likely_log_dump(user_text):
            terminal_context.ingest_text(user_text)
            store.append_assistant(model_key, LOG_ACK_MESSAGE)
            return jsonify({"reply": LOG_ACK_MESSAGE})

        request_messages = with_terminal_context(history, terminal_context.render_context())

        try:
            if provider == "groq":
                reply = groq_backend.chat(request_messages, model_name)
            elif provider == "openai":
                reply = openai_backend.chat(request_messages, model_name)
            elif model_key == EXPERIMENTAL_GPTQ_KEY:
                return jsonify({"error": "GPT-Q 4K is an experimental placeholder and is not selectable in this build."}), 400
            else:
                return jsonify({"error": f"unsupported provider: {provider}"}), 400
        except Exception as exc:  # noqa: BLE001
            return jsonify({"error": str(exc)}), 502

        store.append_assistant(model_key, reply)
        return jsonify({"reply": reply})

    return app


def main() -> None:
    parser = argparse.ArgumentParser(description="Gimble Python chat server")
    parser.add_argument("--port", type=int, default=5555)
    args = parser.parse_args()

    app = create_app()
    localhost_url = f"http://localhost:{args.port}"
    loopback_url = f"http://127.0.0.1:{args.port}"
    print(f"Python chat server listening on {localhost_url}")
    print(f"Python chat server listening on {loopback_url}")

    try:
        from waitress import serve
    except ModuleNotFoundError:
        raise SystemExit(
            f"Missing Python package: waitress\n"
            f"Install dependencies with:\n"
            f"  python3 -m pip install -r {REQ_FILE}"
        )

    serve(app, host="127.0.0.1", port=args.port, threads=8)


if __name__ == "__main__":
    main()
