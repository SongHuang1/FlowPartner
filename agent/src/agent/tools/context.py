import contextvars

# 当前会话 id 上下文：handle_chat 进入时设置，工具执行链（含子智能体）自动继承。
# 基础工具在 client 初始化时注册（无会话上下文），执行时经此获取所属会话，
# 用于 gRPC ExecuteTool / 权限审批的会话关联。
session_id_var: contextvars.ContextVar[str] = contextvars.ContextVar("fp_session_id", default="")


def current_session_id() -> str:
    return session_id_var.get()
