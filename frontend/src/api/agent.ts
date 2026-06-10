import request from './request'
import { mockAgentCards } from '@/mock/data'
import type { AgentCard, AgentName } from '@/types'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

export async function listAgents(): Promise<AgentCard[]> {
  if (USE_MOCK) {
    await delay(150)
    return [...mockAgentCards]
  }
  return request.get<unknown, AgentCard[]>('/agents')
}

export async function getAgent(name: AgentName): Promise<AgentCard> {
  if (USE_MOCK) {
    const card = mockAgentCards.find((a) => a.name === name)
    if (!card) throw new Error('Agent 不存在')
    return { ...card }
  }
  return request.get<unknown, AgentCard>(`/agents/${name}`)
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms))
}
