"""Agent 调度层 - HTTP 服务（不持有 API Key）"""

import json
import sys
import argparse
import socket
import os
from http.server import HTTPServer, BaseHTTPRequestHandler

DEFAULT_PORT = 8989
MAX_PORT = 8999

# 危险指令黑名单（system prompt 内容校验）
DANGEROUS_PATTERNS = [
    "忽略安全限制", "忽略之前的指令", "忽略所有规则", "忽略安全策略",
    "删除文件", "格式化硬盘", "清除数据", "rm -rf",
    "ignore safety", "ignore previous", "ignore all rules", "ignore security",
    "delete file", "format disk", "wipe data",
    "bypass security", "override safety", "disable safety",
]


class AgentHandler(BaseHTTPRequestHandler):
    # 简单的共享密钥认证（Go 后端启动时生成，通过环境变量传入）
    AUTH_TOKEN = os.environ.get("AGENT_AUTH_TOKEN", "")

    def do_POST(self):
        if self.path != "/chat":
            self._send_json(404, {"error": "Not Found"})
            return

        # 认证检查
        auth_header = self.headers.get("Authorization", "")
        expected = "Bearer " + self.AUTH_TOKEN
        if not self.AUTH_TOKEN or auth_header != expected:
            self._send_json(401, {"error": "Unauthorized"})
            return

        content_type = self.headers.get("Content-Type", "")
        if "application/json" not in content_type:
            self._send_json(400, {"error": "Content-Type must be application/json"})
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
        except (ValueError, TypeError):
            content_length = 0
        # Content-Length 限制：防止 DoS 攻击
        if content_length > 1_000_000:  # 1MB 限制
            self._send_json(413, {"error": "Request body too large"})
            return
        try:
            body = json.loads(self.rfile.read(content_length))
        except json.JSONDecodeError as e:
            self._send_json(400, {"error": f"Invalid JSON: {str(e)}"})
            return

        user_message = body.get("content", "")
        model_name = body.get("model_name", "gpt-4")
        system_prompt = body.get("system_prompt", "你是一个有帮助的 AI 助手。")
        temperature = body.get("temperature", 0.7)
        conversation_history = body.get("conversation_history", [])

        try:
            llm_params = self._build_llm_request(
                user_message=user_message,
                model_name=model_name,
                system_prompt=system_prompt,
                temperature=temperature,
                conversation_history=conversation_history,
            )
            self._send_json(200, llm_params)
        except Exception as e:
            error_msg = str(e)
            if "Bearer " in error_msg or "api_key" in error_msg.lower():
                error_msg = "LLM 请求构建失败"
            self._send_json(500, {"error": error_msg})

    def _validate_system_prompt(self, prompt):
        """校验 system prompt 是否包含危险指令，返回 (是否安全, 匹配到的模式)"""
        prompt_lower = prompt.lower()
        for pattern in DANGEROUS_PATTERNS:
            if pattern.lower() in prompt_lower:
                return False, pattern
        return True, None

    def _build_llm_request(self, user_message, model_name, system_prompt,
                           temperature, conversation_history):
        is_safe, matched = self._validate_system_prompt(system_prompt)
        if not is_safe:
            raise ValueError(f"系统提示词包含不安全内容，已被拦截")

        messages = [{"role": "system", "content": system_prompt}]
        messages.extend(conversation_history)
        messages.append({"role": "user", "content": user_message})

        return {
            "model": model_name,
            "messages": messages,
            "temperature": temperature,
        }

    def _send_json(self, status, data):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode("utf-8"))

    def log_message(self, format, *args):
        pass


def find_free_port(start_port, max_port):
    for port in range(start_port, max_port + 1):
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.bind(("127.0.0.1", port))
                return port
        except OSError:
            continue
    raise RuntimeError(f"No free port found in range {start_port}-{max_port}")


def write_port_file(port):
    try:
        port_dir = os.path.expanduser("~/.flowpartner")
        os.makedirs(port_dir, exist_ok=True)
        port_file = os.path.join(port_dir, "agent.port")
        with open(port_file, "w") as f:
            f.write(str(port))
    except OSError as e:
        print(f"Warning: failed to write port file: {e}", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(description="FlowPartner Agent Server")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()

    port = args.port
    try:
        server = HTTPServer((args.host, port), AgentHandler)
    except OSError:
        port = find_free_port(port + 1, MAX_PORT)
        server = HTTPServer((args.host, port), AgentHandler)

    write_port_file(port)

    print(f"Agent server running on {args.host}:{port}", file=sys.stderr)
    sys.stderr.flush()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
        print("Agent server stopped", file=sys.stderr)


if __name__ == "__main__":
    main()
