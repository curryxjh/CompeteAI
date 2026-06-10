import request from './request'

export interface ChatConversation {
  id: string
  title: string
  preview?: string
  createdAt: string
  updatedAt: string
}

export interface ChatMessageRecord {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  payload?: Record<string, unknown>
  createdAt?: string
}

export interface ChatConversationDetail extends ChatConversation {
  messages: ChatMessageRecord[]
}

const ACTIVE_KEY = 'competeai_active_conversation_id'

export function getActiveConversationId(): string | null {
  return localStorage.getItem(ACTIVE_KEY)
}

export function setActiveConversationId(id: string) {
  localStorage.setItem(ACTIVE_KEY, id)
}

export function clearActiveConversationId() {
  localStorage.removeItem(ACTIVE_KEY)
}

export async function listConversations(): Promise<ChatConversation[]> {
  return (await request.get('/chat/conversations')) as ChatConversation[]
}

export async function createConversation(title?: string): Promise<ChatConversation> {
  return (await request.post('/chat/conversations', { title: title ?? '' })) as ChatConversation
}

export async function getConversation(id: string): Promise<ChatConversationDetail> {
  return (await request.get(`/chat/conversations/${id}`)) as ChatConversationDetail
}

export async function deleteConversation(id: string): Promise<void> {
  await request.delete(`/chat/conversations/${id}`)
}

export async function renameConversation(id: string, title: string): Promise<void> {
  await request.patch(`/chat/conversations/${id}`, { title })
}

export async function appendMessages(id: string, messages: ChatMessageRecord[]): Promise<void> {
  await request.post(`/chat/conversations/${id}/messages`, { messages })
}
