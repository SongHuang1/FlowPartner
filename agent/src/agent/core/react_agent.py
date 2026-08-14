import json
import logging


class ReactAgent:
    """ReAct Agent 核心：思考 -> 行动 -> 观察 -> 循环"""

    def __init__(self, session_id: str, call_llm_func, send_event_func, tool_registry):
        self.session_id = session_id
        self.call_llm = call_llm_func
        self.send_event = send_event_func
        self.tools = tool_registry
        self.max_iterations = 10

    async def run(self, user_message: str, history: list = None) -> str:
        """执行完整的 ReAct 循环"""
        if history is None:
            history = []

        messages = [
            {"role": "system", "content": "You are a powerful local AI assistant. You can use tools to read files, write files, browse directories, and more to help users complete various tasks. Please use tools appropriately based on user needs."}
        ]
        messages.extend(history)
        messages.append({"role": "user", "content": user_message})

        tools_def = self.tools.get_openai_tools_definition()

        for iteration in range(self.max_iterations):
            logging.info(f"[ReAct] Thinking round {iteration + 1}")
            await self.send_event(self.session_id, "status_update", {
                "status": "thinking", "iteration": iteration + 1
            })

            payload = json.dumps({
                "model": "auto",
                "messages": messages,
                "tools": tools_def if tools_def else None,
            }, ensure_ascii=False)

            llm_resp = await self.call_llm(self.session_id, payload)

            if not llm_resp.get("success"):
                error_msg = llm_resp.get("error_message", "Unknown error")
                error_guess = llm_resp.get("error_guess", "")
                event_payload = {"message": error_msg}
                if error_guess:
                    event_payload["guess"] = error_guess
                await self.send_event(self.session_id, "error", event_payload)
                return error_msg

            finish_reason = llm_resp.get("finish_reason", "")

            if finish_reason == "tool_calls" and llm_resp.get("tool_calls"):
                assistant_msg = {
                    "role": "assistant",
                    "content": llm_resp.get("content", ""),
                    "tool_calls": llm_resp["tool_calls"]
                }
                messages.append(assistant_msg)

                for tool_call in llm_resp["tool_calls"]:
                    func_name = tool_call["function"]["name"]
                    func_args = json.loads(tool_call["function"]["arguments"])
                    call_id = tool_call["id"]

                    logging.info(f"[ReAct] Calling tool: {func_name}({func_args})")
                    await self.send_event(self.session_id, "tool_call", {
                        "tool": func_name, "args": func_args
                    })

                    result = await self.tools.execute(func_name, func_args)

                    await self.send_event(self.session_id, "tool_result", {
                        "tool": func_name, "result": result[:500]
                    })

                    messages.append({
                        "role": "tool",
                        "tool_call_id": call_id,
                        "content": result
                    })

                continue

            final_answer = llm_resp.get("content", "")
            if finish_reason == "stop" or final_answer:
                logging.info(f"[ReAct] Final answer: {final_answer[:100]}...")
                await self.send_event(self.session_id, "final_answer", {
                    "text": final_answer
                })
                return final_answer

            if not finish_reason:
                await self.send_event(self.session_id, "final_answer", {
                    "text": final_answer,
                    "incomplete": True
                })
                return final_answer

        fallback = "Sorry, I could not reach a conclusion after too many rounds. Please try simplifying your question."
        await self.send_event(self.session_id, "error", {"message": fallback})
        return fallback
