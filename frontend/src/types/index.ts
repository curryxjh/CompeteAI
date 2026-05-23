export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export type AgentName =
  | 'coordinator'
  | 'collector'
  | 'analyst'
  | 'writer'
  | 'qa'

export type AgentRunStatus = 'pending' | 'running' | 'completed' | 'failed' | 'rejected'

export interface AgentState {
  name: AgentName
  status: AgentRunStatus
  progress?: number
  message?: string
  startedAt?: string
  finishedAt?: string
}

export interface Task {
  id: string
  title: string
  competitors: string[]
  dimensions: string[]
  status: TaskStatus
  progress: number
  agentStates: AgentState[]
  createdAt: string
  updatedAt?: string
  errorMessage?: string
}

export interface CreateTaskPayload {
  competitors: string[]
  dimensions: string[]
  title?: string
}

export interface SourceRef {
  url: string
  excerpt: string
  collectedAt: string
  title?: string
}

export interface SWOTItem {
  text: string
  sourceIds?: string[]
}

export interface SWOTAnalysis {
  strengths: SWOTItem[]
  weaknesses: SWOTItem[]
  opportunities: SWOTItem[]
  threats: SWOTItem[]
}

export interface FeatureRow {
  feature: string
  values: Record<string, boolean | string>
  sourceIds?: Record<string, string[]>
}

export interface PricingTier {
  name: string
  price: string
  features: string[]
}

export interface PricingInfo {
  competitor: string
  tiers: PricingTier[]
}

export interface UserPersona {
  competitor: string
  segments: string[]
  painPoints: string[]
  useCases: string[]
}

export interface Report {
  taskId: string
  title: string
  generatedAt: string
  qaScore: number
  summary: string
  swot: Record<string, SWOTAnalysis>
  features: FeatureRow[]
  pricing: PricingInfo[]
  personas: UserPersona[]
  sources: Record<string, SourceRef>
}

export interface TraceNode {
  id: string
  agent: AgentName
  label: string
  status: AgentRunStatus
  durationMs: number
  tokenCount: number
  input?: string
  output?: string
  metadata?: Record<string, string | number>
  isRetry?: boolean
  isRejection?: boolean
  parentId?: string
}

export interface Trace {
  taskId: string
  nodes: TraceNode[]
}

export interface AgentCard {
  name: AgentName
  displayName: string
  description: string
  skills: string[]
  tools: string[]
  dependsOn: AgentName[]
}

export interface AnnotationPayload {
  taskId: string
  section: string
  text: string
  comment: string
}

export interface RejectionEvent {
  fromAgent: AgentName
  toAgent: AgentName
  reason: string
}
