<script setup lang="ts">
import AgentSkillList from './AgentSkillList.vue'
import AgentIcon from '@/components/ui/AgentIcon.vue'
import { AGENT_LABELS } from '@/utils/agent'
import type { AgentCard } from '@/types'

defineProps<{ agents: AgentCard[] }>()
</script>

<template>
  <div class="grid">
    <article
      v-for="(agent, index) in agents"
      :key="agent.name"
      class="agent-card card-interactive panel"
      :style="{ '--stagger': index }"
    >
      <div class="head">
        <AgentIcon :name="agent.name" :size="44" class="avatar" />
        <div>
          <strong>{{ agent.displayName }}</strong>
          <div class="sub font-mono">{{ AGENT_LABELS[agent.name] }}</div>
        </div>
      </div>
      <p class="desc">{{ agent.description }}</p>
      <AgentSkillList :skills="agent.skills" :tools="agent.tools" />
      <div v-if="agent.dependsOn.length" class="deps">
        依赖：
        <el-tag
          v-for="d in agent.dependsOn"
          :key="d"
          size="small"
          type="info"
        >
          {{ AGENT_LABELS[d] }}
        </el-tag>
      </div>
    </article>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.agent-card {
  padding: 18px 20px;
}

.agent-card:hover :deep(.agent-icon) {
  transform: scale(1.06);
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.2);
}

.head {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 14px;
  position: relative;
  z-index: 1;
}

.avatar {
  flex-shrink: 0;
}

.head strong {
  font-size: 15px;
}

.sub {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.55;
  margin: 0 0 12px;
  position: relative;
  z-index: 1;
}

.deps {
  margin-top: 12px;
  font-size: 12px;
  position: relative;
  z-index: 1;
}

.deps :deep(.el-tag) {
  margin-left: 4px;
  transition: transform 0.2s ease;
}

.deps :deep(.el-tag:hover) {
  transform: translateY(-1px);
}
</style>
