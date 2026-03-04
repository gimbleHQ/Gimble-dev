#!/usr/bin/env python3
import argparse
import os
import threading
from pathlib import Path
from typing import Dict, List

try:
    from flask import Flask, jsonify, request, send_from_directory
except ModuleNotFoundError as exc:
    missing = str(exc).split("'")[-2] if "'" in str(exc) else "flask"
    print(f"Missing Python package: {missing}")
    print("Install dependencies with:")
    print("  python3 -m pip install -r " + str((Path(__file__).resolve().parent / "requirements.txt")))
    raise SystemExit(1)


DEFAULT_SYSTEM_PROMPT = "You are Gimble Assistant. Be concise, practical, and clear."
DEFAULT_OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4")
REQ_FILE = Path(__file__).resolve().parent / "requirements.txt"
DEFAULT_LLAMA_LABEL = "LLaMA 3 7B (Local CPU)"
DEFAULT_OPENAI_LABEL = "GPT-4 (OpenAI API)"


def chat_env_path() -> Path:
    if os.name == "nt":
        base = Path(os.environ.get("APPDATA", Path.home()))
        return base / "gimble" / "chat.env"
    if os.uname().sysname.lower() == "darwin":
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


class ConversationStore:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._messages: Dict[str, List[Dict[str, str]]] = {
            "llama": [{"role": "system", "content": DEFAULT_SYSTEM_PROMPT}],
            "gpt4": [{"role": "system", "content": DEFAULT_SYSTEM_PROMPT}],
        }

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
        # Keep system + recent turns to limit CPU/memory usage.
        msgs = self._messages[model_key]
        if len(msgs) > 31:
            self._messages[model_key] = [msgs[0]] + msgs[-30:]


class LlamaBackend:
    def __init__(self) -> None:
        self.label = DEFAULT_LLAMA_LABEL
        cache_dir = Path(os.getenv("GIMBLE_LLAMA_CACHE_DIR", Path.home() / ".cache" / "gimble" / "models"))
        self.model_path = Path(
            os.getenv(
                "GIMBLE_LLAMA_MODEL_PATH",
                cache_dir / "Meta-Llama-3-8B-Instruct-Q4_K_M.gguf",
            )
        )
        self.repo_id = os.getenv("GIMBLE_LLAMA_HF_REPO", "bartowski/Meta-Llama-3-8B-Instruct-GGUF")
        self.repo_file = os.getenv("GIMBLE_LLAMA_HF_FILE", "Meta-Llama-3-8B-Instruct-Q4_K_M.gguf")
        self.auto_download = os.getenv("GIMBLE_LLAMA_AUTO_DOWNLOAD", "1") == "1"
        self.n_ctx = int(os.getenv("GIMBLE_LLAMA_N_CTX", "2048"))
        self.n_threads = int(os.getenv("GIMBLE_LLAMA_THREADS", str(max((os.cpu_count() or 2) - 1, 1))))

        self._lock = threading.Lock()
        self._llm = None

    def available(self) -> bool:
        return True

    def _ensure_model_file(self) -> None:
        if self.model_path.exists():
            return
        if not self.auto_download:
            raise RuntimeError(
                f"LLaMA model missing at {self.model_path}. Set GIMBLE_LLAMA_AUTO_DOWNLOAD=1 or place a GGUF there."
            )
        self.model_path.parent.mkdir(parents=True, exist_ok=True)
        try:
            from huggingface_hub import hf_hub_download
        except ModuleNotFoundError:
            raise RuntimeError(
                f"huggingface-hub is required for auto-download. Run: python3 -m pip install -r {REQ_FILE}"
            )

        print(f"Downloading local model {self.repo_id}/{self.repo_file} ...")
        downloaded = hf_hub_download(
            repo_id=self.repo_id,
            filename=self.repo_file,
            local_dir=str(self.model_path.parent),
            local_dir_use_symlinks=False,
        )
        downloaded_path = Path(downloaded)
        if downloaded_path != self.model_path:
            downloaded_path.replace(self.model_path)

    def _ensure_loaded(self):
        with self._lock:
            if self._llm is not None:
                return self._llm
            self._ensure_model_file()
            try:
                from llama_cpp import Llama
            except ModuleNotFoundError:
                raise RuntimeError(
                    f"llama-cpp-python is required. Run: python3 -m pip install -r {REQ_FILE}"
                )
            print(f"Loading local model from {self.model_path} (this may take a moment)...")
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
            raise RuntimeError(f"LLaMA inference error: {exc}")


class OpenAIBackend:
    def __init__(self) -> None:
        env = load_chat_env()
        self.api_key = os.getenv("OPENAI_API_KEY", env.get("OPENAI_API_KEY", "")).strip()
        self.model = os.getenv("OPENAI_MODEL", env.get("OPENAI_MODEL", DEFAULT_OPENAI_MODEL)).strip() or DEFAULT_OPENAI_MODEL
        self.label = DEFAULT_OPENAI_LABEL

    def available(self) -> bool:
        return bool(self.api_key)

    def chat(self, messages: List[Dict[str, str]]) -> str:
        if not self.api_key:
            raise RuntimeError("OPENAI_API_KEY is not configured. Set it in env or gimble chat.env")
        try:
            from openai import OpenAI
        except ModuleNotFoundError:
            raise RuntimeError(f"openai package is required. Run: python3 -m pip install -r {REQ_FILE}")

        client = OpenAI(api_key=self.api_key)
        try:
            response = client.chat.completions.create(
                model=self.model,
                messages=messages,
                temperature=0.7,
            )
            text = response.choices[0].message.content or ""
            return text.strip() or "(empty response)"
        except Exception as exc:  # noqa: BLE001
            raise RuntimeError(f"OpenAI API error: {exc}")


def create_app() -> Flask:
    app = Flask(__name__, static_folder=str(Path(__file__).resolve().parent / "web"), static_url_path="")

    store = ConversationStore()
    llama = LlamaBackend()
    gpt4 = OpenAIBackend()

    model_registry = {
        "llama": llama,
        "gpt4": gpt4,
    }

    @app.get("/")
    def index():
        return send_from_directory(app.static_folder, "index.html")

    @app.get("/api/models")
    def models():
        return jsonify(
            {
                "default": "llama",
                "models": [
                    {"key": "llama", "label": llama.label, "available": llama.available()},
                    {"key": "gpt4", "label": gpt4.label, "available": gpt4.available()},
                ],
            }
        )

    @app.post("/api/chat")
    def chat():
        payload = request.get_json(silent=True) or {}
        text = (payload.get("message") or "").strip()
        model_key = (payload.get("model") or "llama").strip()

        if not text:
            return jsonify({"error": "message cannot be empty"}), 400
        if model_key not in model_registry:
            return jsonify({"error": f"unknown model: {model_key}"}), 400

        history = store.append_user(model_key, text)
        backend = model_registry[model_key]
        try:
            reply = backend.chat(history)
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
    print(f"Python chat server listening on http://127.0.0.1:{args.port}")
    app.run(host="127.0.0.1", port=args.port, debug=False, threaded=True)


if __name__ == "__main__":
    main()
