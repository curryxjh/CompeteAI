<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ANALYSIS_DIMENSIONS } from '@/mock/data'
import { DEFAULT_ANALYSIS_DIMENSIONS, parseCompetitors } from '@/utils/analysis'
import type { CreateTaskPayload } from '@/types'

const emit = defineEmits<{
  submit: [payload: CreateTaskPayload]
}>()

const prompt = ref('分析 Cursor 与 GitHub Copilot 的功能、定价与 SWOT')
const expanded = ref(false)
const dimensions = ref<string[]>([...DEFAULT_ANALYSIS_DIMENSIONS])
const loading = ref(false)

async function onSubmit() {
  const competitors = parseCompetitors(prompt.value)
  if (competitors.length < 2) {
    expanded.value = true
    ElMessage.warning('请至少输入两个竞品，例如：分析 Cursor 与 GitHub Copilot')
    return
  }
  loading.value = true
  try {
    emit('submit', {
      competitors,
      dimensions: dimensions.value,
      title: prompt.value.slice(0, 80) || undefined,
    })
    prompt.value = ''
    expanded.value = false
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="omnibar-wrap">
    <div class="omnibar" :class="{ expanded, loading }">
      <span class="prompt-char">&gt;</span>
      <textarea
        v-model="prompt"
        class="omnibar-input"
        rows="1"
        placeholder="描述竞品分析任务，例如：分析 Cursor 与 Copilot 的功能对比..."
        @focus="expanded = true"
        @keydown.enter.exact.prevent="onSubmit"
      />
      <button
        type="button"
        class="send-btn"
        :disabled="loading"
        title="发送 (Enter)"
        @click="onSubmit"
      >
        <span v-if="loading" class="spin ring" />
        <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="12" y1="19" x2="12" y2="5"/>
          <polyline points="5 12 12 5 19 12"/>
        </svg>
      </button>
    </div>

    <Transition name="slide">
      <div v-if="expanded" class="omnibar-meta">
        <span class="hint">分析维度</span>
        <el-checkbox-group v-model="dimensions" size="small">
          <el-checkbox
            v-for="d in ANALYSIS_DIMENSIONS"
            :key="d"
            :label="d"
            :value="d"
          />
        </el-checkbox-group>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.omnibar-wrap {
  position: sticky;
  bottom: 0;
  margin-top: 28px;
  padding-top: 16px;
  background: linear-gradient(to top, var(--bg-base) 60%, transparent);
}

.omnibar {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 14px 16px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.omnibar:focus-within {
  border-color: var(--border-active);
  box-shadow: var(--ring-focus);
}

.omnibar.loading {
  animation: pulse-glow 2s ease-in-out infinite;
}

.prompt-char {
  color: var(--accent-light);
  font-size: 16px;
  font-weight: 600;
  padding-bottom: 9px;
  font-family: var(--font-mono);
}

.omnibar-input {
  flex: 1;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.5;
  min-height: 26px;
  max-height: 120px;
  font-family: var(--font-sans);
}

.omnibar-input::placeholder {
  color: var(--text-muted);
}

.send-btn {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--accent-dim);
  color: var(--accent-light);
  cursor: pointer;
  transition: all 0.15s;
}

.send-btn:hover:not(:disabled) {
  border-color: var(--border-active);
  background: var(--accent);
  color: #fff;
}

.send-btn:disabled {
  opacity: 0.5;
}

.ring {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-default);
  border-top-color: var(--accent-light);
  border-radius: 50%;
}

.omnibar-meta {
  margin-top: 12px;
  padding: 12px 4px;
}

.hint {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  margin-bottom: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

:deep(.el-checkbox) {
  color: var(--text-secondary);
  margin-right: 20px;
}

:deep(.el-checkbox.is-checked .el-checkbox__label) {
  color: var(--accent-light);
}

.slide-enter-active,
.slide-leave-active {
  transition: all 0.2s cubic-bezier(0.22, 1, 0.36, 1);
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
