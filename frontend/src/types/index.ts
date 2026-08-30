export interface ToolCall {
  id: string
  type: string
  function: {
    name: string
    arguments: string
  }
}

export interface SubAgentResult {
  span_id: string
  agent_name: string
  task: string
  content: string
  status: 'running' | 'done' | 'error'
}

export type ContentBlock =
  | { type: 'text'; content: string }
  | {
      type: 'subagent'
      span_id: string
      agent_name: string
      task: string
      status: 'running' | 'done' | 'error'
      steps: SubAgentStep[]
      result?: string
      error?: string
    }

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: number
  status?: 'streaming' | 'completed'
  tool_calls?: ToolCall[]
  tool_call_id?: string
  name?: string
  subagent_results?: SubAgentResult[]
  content_blocks?: ContentBlock[]
}

export interface HistoryEntry {
  session_id: string
  title: string
  updated_at: number
  message_count: number
}

export interface SubAgentRef {
  call_id: string
  span_id: string
}

export interface HistoryMessage {
  role: 'user' | 'assistant' | 'tool'
  content: string
  tool_calls?: ToolCall[]
  tool_call_id?: string
  name?: string
  subagent_results?: SubAgentResult[]
  subagent_refs?: SubAgentRef[]
}

export interface HistorySession {
  session_id: string
  messages: HistoryMessage[]
  subagents?: Record<string, SubAgentRun>
}

export interface Settings {
  model: string
  agent_id: string
  context_window: number
  working_directory: string
  language: string

  base_url: string
  encrypted_api_key: string
  model_name: string

  system_prompt: string
  temperature: number

  close_behavior: 'minimize' | 'quit' | 'ask'
  close_remembered: boolean

  window_x: number
  window_y: number
  window_width: number
  window_height: number
  sidebar_visible: boolean
  sidebar_view: string

  trash_dir: string

  snapshot_dir: string
  snapshot_enabled: boolean
  snapshot_include_secrets: boolean
}

export interface SnapshotStatus {
  phase: 'idle' | 'snapshotting' | 'error'
  last_at?: string
  count: number
  size_bytes: number
  skipped_files: number
  queued?: boolean
  error?: string
  last_snapshot_id?: string
}

export interface SnapshotMessage {
  type: 'info' | 'warning' | 'error'
  text: string
}

export interface SkippedFile {
  path: string
  reason: string
  detail?: string
}

export interface SymlinkEntry {
  path: string
  target: string
}

export interface SnapshotManifest {
  snapshot_id: string
  project_id: string
  reason: 'debounce' | 'ticker' | 'lock' | 'manual' | 'prerestore'
  created_at: string
  workspace_root: string
  workspace_root_normalized: string
  file_count: number
  total_size_bytes: number
  complete: boolean
  skipped_files: SkippedFile[]
  symlinks: SymlinkEntry[]
}

export interface ProtectedEntry {
  path: string
  type: 'secret' | 'too_large' | 'excluded_dir'
  detail?: string
}

export interface SnapshotDetail {
  manifest: SnapshotManifest
  protected_files: ProtectedEntry[]
}

export interface LockStatus {
  locked: boolean
  locked_until?: string
  failed_attempts: number
  has_api_key: boolean
}

export interface WindowState {
  x: number
  y: number
  width: number
  height: number
  sidebar_visible: boolean
  sidebar_view: string
}

export interface PermissionRequestPayload {
  request_id: string
  tool: string
  path: string
  operation: string
  detail: string
  scope_options?: string[]
}

export interface IterationStepToolCall {
  tool: string
  args: Record<string, unknown>
  call_id: string
  result?: string
  truncated?: boolean
}

export interface IterationStep {
  iteration: number
  thinking: string
  toolCalls: IterationStepToolCall[]
  loopTerminated?: { reason: string }
}

export interface AgentMeta {
  id: string
  name: string
  description: string
}

export interface AgentDef extends AgentMeta {
  system_prompt: string
  created_at: number
  updated_at: number
}

export interface AgentInput {
  name: string
  description: string
  system_prompt: string
}

export type SubAgentStepType = 'thinking' | 'tool_call' | 'tool_result' | 'final_answer'

export interface SubAgentStep {
  step_type: SubAgentStepType
  content?: string
  tool?: string
  args?: Record<string, unknown>
  result?: string
  truncated?: boolean
}

export type SubAgentStatus = 'running' | 'done' | 'error'

export interface SubAgentRun {
  agent_id: string
  agent_name: string
  depth: number
  span_id: string
  trace_id: string
  parent_span_id?: string
  status: SubAgentStatus
  task?: string
  result?: string
  error?: string
  steps: SubAgentStep[]
}

export interface UsageUpdate {
  input_tokens: number
  cached_input_tokens: number
  output_tokens: number
  reasoning_output_tokens: number
  total_tokens: number
  model_context_window: number
  estimated: boolean
}

export interface TurnInfo {
  thread_id: string
  turn_id: string
  status: 'active' | 'completed' | 'aborted'
  last_agent_message?: string
  duration_ms?: number
}
