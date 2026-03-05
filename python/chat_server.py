#!/usr/bin/env python3
import argparse
import os
import platform
import shutil
import tempfile
import threading
import urllib.request
from pathlib import Path
from typing import Dict, List, Tuple

try:
    from flask import Flask, jsonify, request, send_from_directory
except ModuleNotFoundError as exc:
    missing = str(exc).split("'")[-2] if "'" in str(exc) else "flask"
    req_file = Path(__file__).resolve().parent / "requirements.txt"
    print(f"Missing Python package: {missing}")
    print("Install dependencies with:")
    print(f"  python3 -m pip install -r {req_file}")
    raise SystemExit(1)


DEFAULT_SYSTEM_PROMPT = "You are Gimble Assistant. Be concise, practical, and clear."
REQ_FILE = Path(__file__).resolve().parent / "requirements.txt"

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
DEFAULT_GPTQ4K_FILE = "gptq-4k-quantized.gguf"
DEFAULT_GPTQ4K_URL = (
    "https://huggingface.co/bartowski/Meta-Llama-3-8B-Instruct-GGUF/resolve/main/"
    "Meta-Llama-3-8B-Instruct-Q4_K_M.gguf"
)


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


def is_valid_gguf(path: Path) -> bool:
    if not path.exists() or path.stat().st_size < 4:
        return False
    try:
        with path.open("rb") as f:
            magic = f.read(4)
        return magic == b"GGUF"
    except OSError:
        return False


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


class LlamaCppBackend:
    def __init__(self, *, model_path: Path, auto_download: bool, direct_url: str, n_ctx: int, n_threads: int) -> None:
        self.model_path = model_path
        self.auto_download = auto_download
        self.direct_url = direct_url
        self.n_ctx = n_ctx
        self.n_threads = n_threads
        self._lock = threading.Lock()
        self._llm = None

    def available(self) -> bool:
        return self.model_path.exists() or self.auto_download

    def _download_via_url(self) -> bool:
        if not self.direct_url:
            return False
        with tempfile.NamedTemporaryFile(delete=False, suffix=".gguf") as tmp:
            tmp_path = Path(tmp.name)
        try:
            with urllib.request.urlopen(self.direct_url) as src, tmp_path.open("wb") as dst:
                shutil.copyfileobj(src, dst)
            shutil.move(str(tmp_path), str(self.model_path))
        finally:
            if tmp_path.exists():
                tmp_path.unlink(missing_ok=True)
        return True

    def _ensure_model_file(self) -> None:
        self.model_path.parent.mkdir(parents=True, exist_ok=True)

        if self.model_path.exists() and not is_valid_gguf(self.model_path):
            self.model_path.unlink(missing_ok=True)

        if is_valid_gguf(self.model_path):
            return

        if not self.auto_download:
            raise RuntimeError(f"Model missing at {self.model_path}")

        if self._download_via_url() and is_valid_gguf(self.model_path):
            return

        self.model_path.unlink(missing_ok=True)
        raise RuntimeError(
            f"Could not acquire a valid GGUF model for {EXPERIMENTAL_GPTQ_LABEL}. "
            f"Set {self.model_path} manually or configure GIMBLE_GPTQ4K_URL."
        )

    def _ensure_loaded(self):
        with self._lock:
            if self._llm is not None:
                return self._llm

            self._ensure_model_file()
            try:
                from llama_cpp import Llama
            except ModuleNotFoundError:
                raise RuntimeError(f"llama-cpp-python is required. Run: python3 -m pip install -r {REQ_FILE}")

            self._llm = Llama(
                model_path=str(self.model_path),
                n_ctx=self.n_ctx,
                n_threads=self.n_threads,
                n_gpu_layers=0,
                verbose=False,
            )
            return self._llm

    def chat(self, messages: List[Dict[str, str]]) -> str:
        llm = self._ensure_loaded()
        try:
            result = llm.create_chat_completion(
                messages=messages,
                max_tokens=512,
                temperature=0.7,
            )
            return (result["choices"][0]["message"]["content"] or "").strip() or "(empty response)"
        except Exception as exc:  # noqa: BLE001
            raise RuntimeError(f"{EXPERIMENTAL_GPTQ_LABEL} inference error: {exc}")


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
    gptq_backend = LlamaCppBackend(
        model_path=Path(os.getenv("GIMBLE_GPTQ4K_MODEL_PATH", Path.home() / ".cache" / "gimble" / "models" / DEFAULT_GPTQ4K_FILE)),
        auto_download=os.getenv("GIMBLE_GPTQ4K_AUTO_DOWNLOAD", "1") == "1",
        direct_url=os.getenv("GIMBLE_GPTQ4K_URL", DEFAULT_GPTQ4K_URL),
        n_ctx=int(os.getenv("GIMBLE_LLAMA_N_CTX", "2048")),
        n_threads=int(os.getenv("GIMBLE_LLAMA_THREADS", str(max((os.cpu_count() or 2) - 1, 1)))),
    )

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
            "available": gptq_backend.available(),
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

    session_config = {
        "profile": os.getenv("GIMBLE_PROFILE", ""),
        "name": os.getenv("GIMBLE_USER_NAME", ""),
        "email": os.getenv("GIMBLE_USER_EMAIL", ""),
        "github": os.getenv("GIMBLE_USER_GITHUB", ""),
    }

    @app.get("/")
    def index():
        return send_from_directory(app.static_folder, "index.html")

    @app.get("/api/session-config")
    def session_cfg():
        return jsonify(session_config)

    @app.get("/api/models")
    def models():
        return jsonify({"default": default_key, "models": model_options})

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
        try:
            if provider == "groq":
                reply = groq_backend.chat(history, model_name)
            elif provider == "openai":
                reply = openai_backend.chat(history, model_name)
            elif model_key == EXPERIMENTAL_GPTQ_KEY:
                reply = gptq_backend.chat(history)
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
