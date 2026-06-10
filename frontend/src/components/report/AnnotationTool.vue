<script setup lang="ts">
import { ref } from 'vue'
import { annotateReport } from '@/api/report'
import { ElMessage } from 'element-plus'

const props = defineProps<{ taskId: string }>()

const section = ref('summary')
const text = ref('')
const comment = ref('')
const loading = ref(false)

async function save() {
  if (!comment.value.trim()) return
  loading.value = true
  try {
    await annotateReport({
      taskId: props.taskId,
      section: section.value,
      text: text.value,
      comment: comment.value,
    })
    ElMessage.success('批注已保存')
    comment.value = ''
    text.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-card shadow="never" class="annotate">
    <template #header>人工标注 / 修正</template>
    <el-form label-width="80px">
      <el-form-item label="章节">
        <el-select v-model="section" style="width: 100%">
          <el-option label="概要" value="summary" />
          <el-option label="功能对比" value="features" />
          <el-option label="SWOT" value="swot" />
          <el-option label="定价" value="pricing" />
        </el-select>
      </el-form-item>
      <el-form-item label="选中文本">
        <el-input v-model="text" type="textarea" :rows="2" placeholder="粘贴需要批注的原文" />
      </el-form-item>
      <el-form-item label="批注">
        <el-input v-model="comment" type="textarea" :rows="2" placeholder="您的修正意见" />
      </el-form-item>
      <el-button type="primary" :loading="loading" @click="save">保存批注</el-button>
    </el-form>
  </el-card>
</template>

<style scoped>
.annotate {
  margin-top: 24px;
}
</style>
