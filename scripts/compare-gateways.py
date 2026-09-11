#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import os
import statistics
import sys
import time
import uuid
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

DEFAULT_OLD = "https://lite-llm-deploy-production.up.railway.app"
DEFAULT_NEW = "https://lite-llm-diet-production.up.railway.app"
DEFAULT_MODELS = [
    "openai/gpt-4o-mini",
    "anthropic/claude-haiku-4-5",
    "gemini/gemini-2.5-flash",
]
DIAGNOSTIC = (
    "x-litellm-call-id",
    "x-litellm-model-group",
    "x-litellm-model-name",
    "x-litellm-response-cost",
    "x-litellm-key-spend",
)
class Result:
    def __init__(self, name: str, status: str, detail: str = "") -> None:
        self.name = name
        self.status = status
        self.detail = detail


def color(enabled: bool, code: str, text: str) -> str:
    if not enabled:
        return text
    return f"\033[{code}m{text}\033[0m"


def pct(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    if len(ordered) == 1:
        return ordered[0]
    idx = min(len(ordered) - 1, max(0, round((p / 100) * (len(ordered) - 1))))
    return ordered[idx]


def fmt_ms(seconds: float) -> str:
    return f"{seconds * 1000:.0f}ms"


def clip(text: str, limit: int = 240) -> str:
    text = " ".join(text.split())
    if len(text) <= limit:
        return text
    return text[: limit - 3] + "..."


def parse_json(raw: bytes):
    if not raw:
        return None
    try:
        return json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        return None


class HTTPResult:
    def __init__(
        self,
        status: int,
        headers: dict[str, str],
        body: bytes,
        elapsed: float,
        ttfb: float | None = None,
        error: str = "",
    ) -> None:
        self.status = status
        self.headers = headers
        self.body = body
        self.elapsed = elapsed
        self.ttfb = ttfb if ttfb is not None else elapsed
        self.error = error
        self.json = parse_json(body)
        self.text = body.decode("utf-8", errors="replace")


def header_map(headers) -> dict[str, str]:
    out: dict[str, str] = {}
    for key, value in headers.items():
        out[key.lower()] = value
    return out


def call(
    base: str,
    method: str,
    path: str,
    key: str | None,
    body: dict | None = None,
    timeout: float = 90.0,
    stream: bool = False,
) -> HTTPResult:
    url = base.rstrip("/") + path
    payload = None if body is None else json.dumps(body).encode("utf-8")
    headers = {"Accept": "application/json"}
    if payload is not None:
        headers["Content-Type"] = "application/json"
    if key:
        headers["Authorization"] = "Bearer " + key
    req = Request(url, data=payload, headers=headers, method=method)
    started = time.perf_counter()
    try:
        with urlopen(req, timeout=timeout) as resp:
            hdrs = header_map(resp.headers)
            if stream:
                chunks: list[bytes] = []
                ttfb = None
                while True:
                    piece = resp.read(256)
                    now = time.perf_counter()
                    if ttfb is None:
                        ttfb = now - started
                    if not piece:
                        break
                    chunks.append(piece)
                elapsed = time.perf_counter() - started
                return HTTPResult(resp.status, hdrs, b"".join(chunks), elapsed, ttfb)
            raw = resp.read()
            elapsed = time.perf_counter() - started
            return HTTPResult(resp.status, hdrs, raw, elapsed)
    except HTTPError as exc:
        elapsed = time.perf_counter() - started
        raw = exc.read() if exc.fp is not None else b""
        return HTTPResult(exc.code, header_map(exc.headers), raw, elapsed)
    except URLError as exc:
        elapsed = time.perf_counter() - started
        return HTTPResult(0, {}, b"", elapsed, error=str(exc.reason))
    except TimeoutError:
        elapsed = time.perf_counter() - started
        return HTTPResult(0, {}, b"", elapsed, error="timeout")


def chat_body(model: str, prompt: str, stream: bool = False, extra: dict | None = None) -> dict:
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": 16,
        "temperature": 0,
    }
    if stream:
        payload["stream"] = True
    if extra:
        payload.update(extra)
    return payload


def stream_events(raw: str) -> list[str]:
    events: list[str] = []
    for line in raw.splitlines():
        if line.startswith("data:"):
            events.append(line[5:].strip())
    return events


def model_ids(payload) -> list[str]:
    if not isinstance(payload, dict):
        return []
    data = payload.get("data")
    if not isinstance(data, list):
        return []
    ids: list[str] = []
    for item in data:
        if isinstance(item, dict):
            value = item.get("id") or item.get("model_name") or item.get("model")
            if isinstance(value, str):
                ids.append(value)
    return ids


def has_usage(payload) -> bool:
    if not isinstance(payload, dict):
        return False
    usage = payload.get("usage")
    if not isinstance(usage, dict):
        return False
    return all(key in usage for key in ("prompt_tokens", "completion_tokens", "total_tokens"))


def choice_role(payload) -> str:
    try:
        return payload["choices"][0]["message"]["role"]
    except (TypeError, KeyError, IndexError):
        return ""


def choice_content(payload) -> str:
    try:
        content = payload["choices"][0]["message"]["content"]
        return content if isinstance(content, str) else ""
    except (TypeError, KeyError, IndexError):
        return ""


def stats_line(name: str, samples: list[float]) -> str:
    if not samples:
        return f"{name}: sem amostras"
    return (
        f"{name}: n={len(samples)} min={fmt_ms(min(samples))} "
        f"p50={fmt_ms(pct(samples, 50))} p95={fmt_ms(pct(samples, 95))} "
        f"avg={fmt_ms(statistics.mean(samples))} max={fmt_ms(max(samples))}"
    )


def faster(old: list[float], new: list[float]) -> str:
    if not old or not new:
        return "sem comparação"
    a = statistics.mean(old)
    b = statistics.mean(new)
    if a <= 0:
        return "n/a"
    delta = ((a - b) / a) * 100
    if delta > 2:
        return f"diet {delta:.0f}% mais rápido (média)"
    if delta < -2:
        return f"diet {abs(delta):.0f}% mais lento (média)"
    return "latência média equivalente"


class Runner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.old = args.old.rstrip("/")
        self.new = args.new.rstrip("/")
        self.key = args.key
        self.models = args.models
        self.rounds = args.rounds
        self.timeout = args.timeout
        self.skip_chat = args.skip_chat
        self.bust_cache = not args.no_bust_cache
        self.color = args.color == "always" or (args.color == "auto" and sys.stdout.isatty())
        self.results: list[Result] = []

    def add(self, name: str, status: str, detail: str = "") -> None:
        self.results.append(Result(name, status, detail))
        mark = {
            "pass": color(self.color, "32", "PASS"),
            "fail": color(self.color, "31", "FAIL"),
            "warn": color(self.color, "33", "WARN"),
            "skip": color(self.color, "36", "SKIP"),
        }[status]
        extra = f"  {detail}" if detail else ""
        print(f"  [{mark}] {name}{extra}")

    def both(self, method: str, path: str, body: dict | None = None, key: str | None = None, stream: bool = False):
        token = self.key if key is None else key
        a = call(self.old, method, path, token, body, self.timeout, stream)
        b = call(self.new, method, path, token, body, self.timeout, stream)
        return a, b

    def health(self) -> None:
        print("\n== health ==")
        probes = [
            ("/health/live", "diet"),
            ("/health/ready", "diet"),
            ("/health/liveliness", "litellm"),
            ("/health/liveness", "litellm"),
            ("/health/readiness", "litellm"),
        ]
        old_ok = False
        new_ok = False
        for path, owner in probes:
            a, b = self.both("GET", path, key="")
            if a.status == 200:
                old_ok = True
            if b.status == 200:
                new_ok = True
            self.add(
                f"{path} ({owner})",
                "pass" if a.status == 200 or b.status == 200 else "warn",
                f"old={a.status}/{fmt_ms(a.elapsed)} new={b.status}/{fmt_ms(b.elapsed)}",
            )
        self.add("pelo menos um health 200 no lite-llm", "pass" if old_ok else "fail")
        self.add("pelo menos um health 200 no diet", "pass" if new_ok else "fail")

    def auth(self) -> None:
        print("\n== autenticação ==")
        body = chat_body(self.models[0], "ping")
        missing_old, missing_new = self.both("POST", "/v1/chat/completions", body, key="")
        bad_old, bad_new = self.both("POST", "/v1/chat/completions", body, key="sk-invalid-compare-key")
        for name, a, b, want in (
            ("sem Authorization", missing_old, missing_new, 401),
            ("chave inexistente", bad_old, bad_new, 401),
        ):
            same = a.status == b.status
            expected = a.status == want and b.status == want
            status = "pass" if expected else ("warn" if same else "fail")
            self.add(
                name,
                status,
                f"old={a.status} new={b.status} esperado={want}",
            )
            for side, resp in (("old", a), ("new", b)):
                if isinstance(resp.json, dict) and "error" in resp.json:
                    continue
                if resp.status == want:
                    self.add(f"{name} envelope de erro ({side})", "warn", clip(resp.text))

    def catalog(self) -> None:
        print("\n== catálogo ==")
        for path in ("/v1/models", "/models", "/v1/model/info", "/model/info"):
            a, b = self.both("GET", path)
            same_status = a.status == b.status
            status = "pass" if same_status and a.status == 200 else ("warn" if same_status else "fail")
            self.add(
                path,
                status,
                f"old={a.status}/{fmt_ms(a.elapsed)} new={b.status}/{fmt_ms(b.elapsed)}",
            )

        old_models = set(model_ids(call(self.old, "GET", "/v1/models", self.key, timeout=self.timeout).json))
        new_models = set(model_ids(call(self.new, "GET", "/v1/models", self.key, timeout=self.timeout).json))
        overlap = old_models & new_models
        only_old = sorted(old_models - new_models)
        only_new = sorted(new_models - old_models)
        self.add(
            "interseção de modelos",
            "pass" if overlap or not old_models else "warn",
            f"old={len(old_models)} new={len(new_models)} overlap={len(overlap)}",
        )
        if only_old:
            self.add("só no lite-llm", "warn", ", ".join(only_old[:12]))
        missing_defaults = [m for m in self.models if m not in new_models and new_models]
        if missing_defaults:
            self.add("modelos do teste ausentes no diet", "fail", ", ".join(missing_defaults))
        else:
            self.add("modelos do teste presentes no diet", "pass", ", ".join(self.models))
        info = call(self.new, "GET", "/v1/model/info", self.key, timeout=self.timeout)
        if isinstance(info.json, dict):
            sample = (info.json.get("data") or [None])[0]
            ok = isinstance(sample, dict) and "model_info" in sample
            self.add("model/info com metadados", "pass" if ok else "warn")

    def assert_chat_contract(self, label: str, model: str, a: HTTPResult, b: HTTPResult) -> None:
        if a.status != b.status:
            self.add(f"{label} status", "fail", f"old={a.status} new={b.status} {clip(b.text or a.text)}")
            return
        if a.status != 200:
            self.add(f"{label} status", "fail", f"ambos={a.status} old={clip(a.text)} new={clip(b.text)}")
            return
        self.add(f"{label} status", "pass", f"200 old={fmt_ms(a.elapsed)} new={fmt_ms(b.elapsed)}")

        for side, resp in (("old", a), ("new", b)):
            payload = resp.json if isinstance(resp.json, dict) else {}
            checks = [
                (payload.get("object") == "chat.completion", "object"),
                (payload.get("model") == model, f"model={payload.get('model')!r}"),
                (choice_role(payload) == "assistant", "role"),
                (bool(choice_content(payload)), "content"),
                (has_usage(payload), "usage"),
            ]
            missing = [name for ok, name in checks if not ok]
            if missing:
                self.add(f"{label} contrato ({side})", "fail", ", ".join(missing))
            else:
                self.add(f"{label} contrato ({side})", "pass")

        for header in DIAGNOSTIC:
            ha = header in a.headers
            hb = header in b.headers
            if ha and hb:
                self.add(f"{label} header {header}", "pass")
            elif hb and not ha:
                self.add(f"{label} header {header}", "warn", "só no diet")
            else:
                self.add(f"{label} header {header}", "fail", f"old={ha} new={hb}")

    def chat(self) -> None:
        print("\n== chat completions ==")
        if self.skip_chat:
            self.add("chat", "skip", "--skip-chat")
            return
        for model in self.models:
            prompt = "Reply with the single word pong."
            if self.bust_cache:
                prompt += f" id={uuid.uuid4()}"
            body = chat_body(model, prompt)
            a, b = self.both("POST", "/v1/chat/completions", body)
            self.assert_chat_contract(model, model, a, b)
            alias_a, alias_b = self.both("POST", "/chat/completions", body)
            if alias_a.status == a.status and alias_b.status == b.status:
                self.add(f"{model} caminho /chat/completions", "pass", f"old={alias_a.status} new={alias_b.status}")
            else:
                self.add(
                    f"{model} caminho /chat/completions",
                    "fail",
                    f"v1 old/new={a.status}/{b.status} alias={alias_a.status}/{alias_b.status}",
                )

        drop = chat_body(
            "anthropic/claude-haiku-4-5",
            "Reply with the single word pong.",
            extra={"presence_penalty": 0, "frequency_penalty": 0, "top_p": 1},
        )
        if self.bust_cache:
            drop["messages"][0]["content"] += f" id={uuid.uuid4()}"
        da, db = self.both("POST", "/v1/chat/completions", drop)
        if da.status == 200 and db.status == 200:
            self.add("anthropic drop_params (penalty/top_p)", "pass")
        else:
            self.add(
                "anthropic drop_params (penalty/top_p)",
                "fail",
                f"old={da.status} {clip(da.text)} | new={db.status} {clip(db.text)}",
            )

    def streaming(self) -> None:
        print("\n== streaming ==")
        if self.skip_chat:
            self.add("streaming", "skip", "--skip-chat")
            return
        model = self.models[0]
        prompt = "Reply with the single word pong."
        if self.bust_cache:
            prompt += f" id={uuid.uuid4()}"
        body = chat_body(model, prompt, stream=True)
        a, b = self.both("POST", "/v1/chat/completions", body, stream=True)
        if a.status != 200 or b.status != 200:
            self.add("stream status", "fail", f"old={a.status} {clip(a.text)} | new={b.status} {clip(b.text)}")
            return
        self.add(
            "stream status",
            "pass",
            f"old ttfb={fmt_ms(a.ttfb)} total={fmt_ms(a.elapsed)} | new ttfb={fmt_ms(b.ttfb)} total={fmt_ms(b.elapsed)}",
        )
        for side, resp in (("old", a), ("new", b)):
            ctype = resp.headers.get("content-type", "")
            events = stream_events(resp.text)
            done = bool(events) and events[-1] == "[DONE]"
            parsed = [parse_json(item.encode()) for item in events if item and item != "[DONE]"]
            has_chunk = any(isinstance(item, dict) and item.get("object") == "chat.completion.chunk" for item in parsed)
            finish = any(
                isinstance(item, dict)
                and item.get("choices")
                and item["choices"][0].get("finish_reason")
                for item in parsed
            )
            leaked_usage = any(isinstance(item, dict) and item.get("usage") for item in parsed)
            if "text/event-stream" in ctype:
                self.add(f"stream content-type ({side})", "pass", ctype)
            else:
                self.add(f"stream content-type ({side})", "warn", ctype or "ausente")
            self.add(f"stream chunks ({side})", "pass" if events else "fail", f"n={len(events)}")
            self.add(f"stream [DONE] ({side})", "pass" if done else "fail")
            self.add(f"stream object chunk ({side})", "pass" if has_chunk else "fail")
            self.add(f"stream finish_reason ({side})", "pass" if finish else "warn")
            self.add(
                f"stream sem usage implícito ({side})",
                "pass" if not leaked_usage else "warn",
                "usage presente sem stream_options" if leaked_usage else "",
            )

        used = chat_body(
            model,
            prompt + " usage",
            stream=True,
            extra={"stream_options": {"include_usage": True}},
        )
        ua, ub = self.both("POST", "/v1/chat/completions", used, stream=True)
        for side, resp in (("old", ua), ("new", ub)):
            events = stream_events(resp.text)
            parsed = [parse_json(item.encode()) for item in events if item and item != "[DONE]"]
            has_usage_chunk = any(isinstance(item, dict) and item.get("usage") for item in parsed)
            self.add(
                f"stream include_usage ({side})",
                "pass" if resp.status == 200 and has_usage_chunk else "warn",
                f"status={resp.status} usage={has_usage_chunk}",
            )

    def errors(self) -> None:
        print("\n== erros ==")
        cases = [
            ("provider desconhecido", chat_body("mistral/x", "ping"), 400),
            ("body inválido", None, 400),
        ]
        for name, body, want in cases:
            if body is None:
                a = call_raw_invalid(self.old, self.key, self.timeout)
                b = call_raw_invalid(self.new, self.key, self.timeout)
            else:
                a, b = self.both("POST", "/v1/chat/completions", body)
            same = a.status == b.status
            expected = a.status == want and b.status == want
            self.add(
                name,
                "pass" if expected else ("warn" if same else "fail"),
                f"old={a.status} new={b.status} esperado={want}",
            )
            for side, resp in (("old", a), ("new", b)):
                ok = isinstance(resp.json, dict) and isinstance(resp.json.get("error"), dict)
                self.add(f"{name} envelope ({side})", "pass" if ok else "warn", clip(resp.text))

    def latency(self) -> dict:
        print("\n== latência ==")
        report: dict[str, dict[str, list[float]]] = {}
        if self.skip_chat:
            self.add("latência chat", "skip", "--skip-chat")
            return report

        health_old: list[float] = []
        health_new: list[float] = []
        for _ in range(self.rounds):
            a = call(self.old, "GET", "/health/liveliness", "", timeout=self.timeout)
            if a.status != 200:
                a = call(self.old, "GET", "/health/live", "", timeout=self.timeout)
            b = call(self.new, "GET", "/health/live", "", timeout=self.timeout)
            if a.status == 200:
                health_old.append(a.elapsed)
            if b.status == 200:
                health_new.append(b.elapsed)
        print("  " + stats_line("health old", health_old))
        print("  " + stats_line("health new", health_new))
        print("  " + faster(health_old, health_new))
        report["health"] = {"old": health_old, "new": health_new}

        for model in self.models:
            old_s: list[float] = []
            new_s: list[float] = []
            for i in range(self.rounds):
                prompt = "Reply with the single word pong."
                if self.bust_cache:
                    prompt += f" id={uuid.uuid4()}-{i}"
                body = chat_body(model, prompt)
                a, b = self.both("POST", "/v1/chat/completions", body)
                if a.status == 200:
                    old_s.append(a.elapsed)
                else:
                    self.add(f"latência {model} old[{i}]", "fail", f"{a.status} {clip(a.text)}")
                if b.status == 200:
                    new_s.append(b.elapsed)
                else:
                    self.add(f"latência {model} new[{i}]", "fail", f"{b.status} {clip(b.text)}")
            print("  " + stats_line(f"{model} old", old_s))
            print("  " + stats_line(f"{model} new", new_s))
            print("  " + faster(old_s, new_s))
            report[model] = {"old": old_s, "new": new_s}
            if old_s and new_s:
                self.add(f"latência {model}", "pass", faster(old_s, new_s))
            else:
                self.add(f"latência {model}", "fail", "sem amostras suficientes")
        return report

    def key_info(self) -> None:
        print("\n== chave ==")
        a, b = self.both("GET", "/key/info")
        if a.status == b.status == 200:
            self.add("/key/info", "pass", f"old={fmt_ms(a.elapsed)} new={fmt_ms(b.elapsed)}")
            return
        if a.status == 200 and b.status != 200:
            self.add("/key/info", "warn", f"old={a.status} new={b.status} {clip(b.text)}")
            return
        self.add("/key/info", "warn", f"old={a.status} new={b.status}")

    def summary(self, latency: dict, json_path: str | None) -> int:
        print("\n== resumo ==")
        counts = {key: 0 for key in ("pass", "fail", "warn", "skip")}
        for item in self.results:
            counts[item.status] += 1
        print(
            f"  pass={counts['pass']} fail={counts['fail']} warn={counts['warn']} skip={counts['skip']}"
        )
        failed = [item for item in self.results if item.status == "fail"]
        if failed:
            print("  falhas:")
            for item in failed:
                print(f"    - {item.name}: {item.detail}")
        if json_path:
            payload = {
                "old": self.old,
                "new": self.new,
                "models": self.models,
                "rounds": self.rounds,
                "results": [item.__dict__ for item in self.results],
                "latency": {
                    name: {
                        side: [round(v * 1000, 1) for v in values]
                        for side, values in sides.items()
                    }
                    for name, sides in latency.items()
                },
            }
            with open(json_path, "w", encoding="utf-8") as fh:
                json.dump(payload, fh, indent=2)
            print(f"  relatório: {json_path}")
        return 1 if failed else 0


def call_raw_invalid(base: str, key: str, timeout: float) -> HTTPResult:
    url = base.rstrip("/") + "/v1/chat/completions"
    headers = {"Authorization": "Bearer " + key, "Content-Type": "application/json"}
    req = Request(url, data=b"{", headers=headers, method="POST")
    started = time.perf_counter()
    try:
        with urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            return HTTPResult(resp.status, header_map(resp.headers), raw, time.perf_counter() - started)
    except HTTPError as exc:
        raw = exc.read() if exc.fp is not None else b""
        return HTTPResult(exc.code, header_map(exc.headers), raw, time.perf_counter() - started)
    except URLError as exc:
        return HTTPResult(0, {}, b"", time.perf_counter() - started, error=str(exc.reason))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Compara lite-llm e litellm-diet em produção")
    parser.add_argument("--old", default=os.environ.get("LITELLM_URL", DEFAULT_OLD))
    parser.add_argument("--new", default=os.environ.get("LITELLM_DIET_URL", DEFAULT_NEW))
    parser.add_argument("--key", default=os.environ.get("LITELLM_API_KEY", ""))
    parser.add_argument("--models", nargs="+", default=DEFAULT_MODELS)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--timeout", type=float, default=90.0)
    parser.add_argument("--skip-chat", action="store_true")
    parser.add_argument("--no-bust-cache", action="store_true")
    parser.add_argument("--json", dest="json_path", default="")
    parser.add_argument("--color", choices=("auto", "always", "never"), default="auto")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.key:
        print("defina LITELLM_API_KEY ou passe --key", file=sys.stderr)
        return 2
    print("comparando")
    print(f"  lite-llm      {args.old}")
    print(f"  lite-llm-diet {args.new}")
    print(f"  modelos       {', '.join(args.models)}")
    print(f"  rounds        {args.rounds}")
    runner = Runner(args)
    runner.health()
    runner.auth()
    runner.catalog()
    runner.errors()
    runner.key_info()
    runner.chat()
    runner.streaming()
    latency = runner.latency()
    return runner.summary(latency, args.json_path or None)


if __name__ == "__main__":
    sys.exit(main())
