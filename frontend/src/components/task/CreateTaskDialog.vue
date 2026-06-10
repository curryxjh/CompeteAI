<script setup lang="ts">
import { ref } from 'vue'
import { ANALYSIS_DIMENSIONS } from '@/mock/data'
import type { CreateTaskPayload } from '@/types'

const visible = defineModel<boolean>('visible', { default: false })
const emit = defineEmits<{
  submit: [payload: CreateTaskPayload]
}>()

const competitorsText = ref('Cursor, GitHub Copilot')
const dimensions = ref<string[]>(['功能对比', 'SWOT', '定价'])
const title = ref('')
const loading = ref(false)

function parseCompetitors(): string[] {
  return competitorsText.value
    .split(/[,，、\n]/)
    .map((s) => s.trim())
    .filter(Boolean)
}

async function onSubmit() {
  const competitors = parseCompetitors()
  if (competitors.length < 2) {
    return
  }
  loading.value = true
  try {
    emit('submit', {
      competitors,
      dimensions: dimensions.value,
      title: title.value || undefined,
    })
    visible.value = false
    competitorsText.value = ''
    title.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="新建竞品分析任务"
    width="520px"
    destroy-on-close
  >
    <el-form label-width="100px" @submit.prevent="onSubmit">
      <el-form-item label="竞品名称" required>
        <el-input
          v-model="competitorsText"
          type="textarea"
          :rows="2"
          placeholder="多个竞品用逗号分隔，如：Cursor, GitHub Copilot"
        />
      </el-form-item>
      <el-form-item label="任务标题">
        <el-input
          v-model="title"
          placeholder="留空则自动生成"
        />
      </el-form-item>
      <el-form-item label="分析维度" required>
        <el-checkbox-group v-model="dimensions">
          <el-checkbox
            v-for="d in ANALYSIS_DIMENSIONS"
            :key="d"
            :label="d"
            :value="d"
          />
        </el-checkbox-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="parseCompetitors().length < 2 || dimensions.length === 0"
        @click="onSubmit"
      >
        开始分析
      </el-button>
    </template>
  </el-dialog>
</template>
