import json
import logging

class ReactAgent:
    """ReAct Agent 核心：思考 -> 行动 -> 观察 -> 循环"""

    def __init__(self, session_id: str, call_llm_func, send_event_func, tool_registry):
        self.session_id = session_id
        self.call_llm = call_llm_func      # 异步函数：请求 Go 调用大模型
        self.send_event = send_event_func   # 异步函数：向 Go 发送实时事件
        self.tools = tool_registry
        self.max_iterations = 10            # 防止无限循环的安全阀

    async def run(self, user_message: str, history: list = None) -> str:
        """执行完整的 ReAct 循环"""
        if history is None:
            history = []

        # 构建初始消息列表
        messages = [
            {"role": "system", "content": "你是一个强大的本地智能助手。你可以使用工具来读取文件、写入文件、浏览目录等，帮助用户完成各种任务。请根据用户需求，合理使用工具。"}
        ]
        messages.extend(history)
        messages.append({"role": "user", "content": user_message})

        tools_def = self.tools.get_openai_tools_definition()

        for iteration in range(self.max_iterations):
            logging.info(f"[ReAct] 第 {iteration + 1} 轮思考")
            await self.send_event(self.session_id, "status_update", {
                "status": "thinking", "iteration": iteration + 1
            })

            # 1. 组装 Payload，请求 Go 代为调用 LLM
            payload = json.dumps({
                "model": "auto",  # Go 端会替换为用户配置的真实模型
                "messages": messages,
                "tools": tools_def if tools_def else None,
            }, ensure_ascii=False)

            llm_resp = await self.call_llm(self.session_id, payload)
            if not llm_resp.get("success"):
                error_msg = f"LLM 调用失败: {llm_resp.get('error_message')}"
                await self.send_event(self.session_id, "error", {"message": error_msg})
                return error_msg

            # 2. 解析 LLM 响应 (OpenAI 格式)
            response_data = json.loads(llm_resp["json_response"])
            choice = response_data.get("choices", [{}])[0]
            assistant_msg = choice.get("message", {})
            finish_reason = choice.get("finish_reason", "")

            # 3. 判断：LLM 是否要求调用工具？
            if finish_reason == "tool_calls" and "tool_calls" in assistant_msg:
                # 把 assistant 的 tool_calls 消息加入上下文
                messages.append(assistant_msg)

                for tool_call in assistant_msg["tool_calls"]:
                    func_name = tool_call["function"]["name"]
                    func_args = json.loads(tool_call["function"]["arguments"])
                    call_id = tool_call["id"]

                    logging.info(f"[ReAct] 调用工具: {func_name}({func_args})")
                    await self.send_event(self.session_id, "tool_call", {
                        "tool": func_name, "args": func_args
                    })

                    # 执行本地工具
                    result = await self.tools.execute(func_name, func_args)

                    await self.send_event(self.session_id, "tool_result", {
                        "tool": func_name, "result": result[:500]  # 截断避免日志过长
                    })

                    # 把工具执行结果加入上下文，供 LLM 下一轮思考
                    messages.append({
                        "role": "tool",
                        "tool_call_id": call_id,
                        "content": result
                    })

                continue  # 继续下一轮 ReAct 循环

            # 4. LLM 给出了最终回答
            if finish_reason == "stop" or "content" in assistant_msg:
                final_answer = assistant_msg.get("content", "")
                logging.info(f"[ReAct] 最终回答: {final_answer[:100]}...")
                await self.send_event(self.session_id, "final_answer", {
                    "text": final_answer
                })
                return final_answer

        # 安全阀触发
        fallback = "抱歉，我思考了太多轮仍未得出结论，请尝试简化您的问题。"
        await self.send_event(self.session_id, "error", {"message": fallback})
        return fallback