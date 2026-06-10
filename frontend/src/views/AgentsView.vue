<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { listAgents } from '@/api/agent'
import AgentCardGrid from '@/components/agents/AgentCardGrid.vue'
import AgentDAGGraph from '@/components/agents/AgentDAGGraph.vue'
import type { AgentCard } from '@/types'

const agents = ref<AgentCard[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    agents.value = await listAgents()
  } catch {
    agents.value = []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-loading="loading" class="agents-view">
    <section class="intro panel">
      <p class="eyebrow">Agent 协议</p>
      <h3>通信与编排架构</h3>
      <p class="intro-text">
        A2A 结构化消息 · Kafka 异步传递 · Coordinator DAG 编排 · QA 打回闭环
      </p>
    </section>

    <h3 class="section-title">Agent 列表</h3>
    <AgentCardGrid :agents="agents" />

    <h3 class="section-title">依赖图</h3>
    <AgentDAGGraph v-if="agents.length" :agents="agents" />
  </div>
</template>

<style scoped>
.agents-view {
  max-width: 1000px;
  margin: 0 auto;
}

.intro {
  padding: 24px 26px;
  margin-bottom: 28px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.eyebrow {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-light);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.intro h3 {
  margin: 0 0 10px;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.intro-text {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.7;
}

.section-title {
  margin: 28px 0 14px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
</style>
