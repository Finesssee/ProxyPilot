import argparse
import base64
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs


def encode_claims(exp, email, account_id, plan_type):
    payload = {
        "email": email,
        "exp": exp,
        "https://api.openai.com/auth": {
            "chatgpt_account_id": account_id,
            "chatgpt_plan_type": plan_type,
        },
    }
    raw = json.dumps(payload, separators=(",", ":")).encode()
    return base64.urlsafe_b64encode(raw).decode().rstrip("=")


class ReusableServer(ThreadingHTTPServer):
    allow_reuse_address = True


class Handler(BaseHTTPRequestHandler):
    scenario = "preemptive"
    log_path = None
    state = {"model_calls": 0}

    def log_message(self, format, *args):
        return

    def _write_json(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        return body

    def _record(self, *, response_status, response_body, request_body=""):
        entry = {
            "ts": time.time(),
            "scenario": self.scenario,
            "method": self.command,
            "path": self.path,
            "headers": {k: v for k, v in self.headers.items()},
            "request_body": request_body,
            "response_status": response_status,
            "response_body": response_body,
            "model_call_index": self.state["model_calls"],
        }
        with open(self.log_path, "a", encoding="utf-8") as fh:
            fh.write(json.dumps(entry) + "\n")

    def do_GET(self):
        if self.path == "/__health":
            body = b"ok"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return

        if self.path == "/v1/models":
            auth = self.headers.get("Authorization", "")
            self.state["model_calls"] += 1
            if self.scenario == "retry401" and self.state["model_calls"] == 1:
                payload = {
                    "error": "expired token",
                    "received_authorization": auth,
                    "attempt": 1,
                }
                self._record(response_status=401, response_body=payload)
                self._write_json(401, payload)
                return
            payload = {
                "object": "list",
                "data": [{"id": "gpt-5.2-codex", "object": "model"}],
                "received_authorization": auth,
                "attempt": self.state["model_calls"],
                "scenario": self.scenario,
            }
            self._record(response_status=200, response_body=payload)
            self._write_json(200, payload)
            return

        payload = {"error": "not found", "path": self.path}
        self._record(response_status=404, response_body=payload)
        self._write_json(404, payload)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw_body = self.rfile.read(length).decode()
        if self.path == "/oauth/token":
            form = parse_qs(raw_body)
            refresh_token = form.get("refresh_token", [""])[0]
            if self.scenario == "preemptive":
                payload = {
                    "access_token": "preemptive-fresh-token",
                    "refresh_token": "preemptive-fresh-refresh-token",
                    "id_token": f"header.{encode_claims(1893456000, 'refresh@example.com', 'acct_preemptive', 'team')}.sig",
                    "expires_in": 3600,
                    "seen_refresh_token": refresh_token,
                }
            else:
                payload = {
                    "access_token": "retry-fresh-token",
                    "refresh_token": "retry-fresh-refresh-token",
                    "id_token": f"header.{encode_claims(1893456000, 'refresh@example.com', 'acct_retry401', 'pro')}.sig",
                    "expires_in": 3600,
                    "seen_refresh_token": refresh_token,
                }
            self._record(response_status=200, response_body=payload, request_body=raw_body)
            self._write_json(200, payload)
            return

        payload = {"error": "not found", "path": self.path}
        self._record(response_status=404, response_body=payload, request_body=raw_body)
        self._write_json(404, payload)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--scenario", required=True, choices=["preemptive", "retry401"])
    parser.add_argument("--port", required=True, type=int)
    parser.add_argument("--log", required=True)
    args = parser.parse_args()

    Handler.scenario = args.scenario
    Handler.log_path = args.log
    Handler.state = {"model_calls": 0}
    server = ReusableServer(("127.0.0.1", args.port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
