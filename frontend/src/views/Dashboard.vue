<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useTaskStore } from '@/stores/task'
import Omnibar from '@/components/task/Omnibar.vue'
import TaskCard from '@/components/task/TaskCard.vue'
import type { CreateTaskPayload } from '@/types'

const store = useTaskStore()
const filterOpen = ref(false)

onMounted(() => store.fetchTasks())

async function onCreate(payload: CreateTaskPayload) {
  await store.create(payload)
}
</script>

<template>
  <div class="dashboard">
    <header class="dash-head">
      <div>
        <p class="eyebrow">竞品分析</p>
        <h2>任务管理</h2>
        <p class="sub">Agent 协作执行 · 全链路可追踪</p>
      </div>
      <button type="button" class="btn-ghost" @click="filterOpen = !filterOpen">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
        </svg>
        筛选
        <span class="filter-indicator">{{ filterOpen ? '−' : '+' }}</span>
      </button>
    </header>

    <Transition name="fade">
      <div v-if="filterOpen" class="filters panel">
        <el-input
          v-model="store.searchQuery"
          placeholder="搜索任务..."
          clearable
          class="filter-input"
        />
        <el-select
          v-model="store.filterStatus"
          placeholder="全部状态"
          clearable
          class="filter-select"
        >
          <el-option label="进行中" value="running" />
          <el-option label="已完成" value="completed" />
          <el-option label="失败" value="failed" />
        </el-select>
      </div>
    </Transition>

    <el-skeleton v-if="store.loading" :rows="4" animated />

    <TransitionGroup
      v-else
      name="card-list"
      tag="div"
      class="task-list"
    >
      <div v-if="!store.filteredTasks().length" key="empty" class="empty panel card-interactive no-lift">
        <div class="empty-icon">
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/>
            <rect x="9" y="3" width="6" height="4" rx="1"/>
            <path d="M12 11v6M9 14h6"/>
          </svg>
        </div>
        <p class="empty-title">暂无任务</p>
        <p class="empty-hint">在下方输入框描述分析目标，启动 Agent 流水线</p>
      </div>
      <TaskCard
        v-for="(task, index) in store.filteredTasks()"
        :key="task.id"
        :task="task"
        :index="index"
      />
    </TransitionGroup>

    <Omnibar @submit="onCreate" />
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 880px;
  margin: 0 auto;
  padding-bottom: 32px;
}

.dash-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-light);
  letter-spacing: 0.03em;
  text-transform: uppercase;
}

.dash-head h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.sub {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--text-muted);
}

.filter-indicator {
  font-family: var(--font-mono);
  font-size: 11px;
  opacity: 0.6;
}

.filters {
  display: flex;
  gap: 12px;
  padding: 14px;
  margin-bottom: 20px;
}

.filter-input {
  flex: 1;
  max-width: 300px;
}

.filter-select {
  width: 140px;
}

.task-list {
  position: relative;
}

.empty {
  padding: 48px 24px;
  text-align: center;
  margin-bottom: 12px;
  background: var(--bg-elevated);
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-lg);
}

.empty-icon {
  color: var(--text-muted);
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
}

.empty-hint {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s, transform 0.15s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
