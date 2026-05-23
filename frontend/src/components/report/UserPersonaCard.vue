<script setup lang="ts">
import type { UserPersona } from '@/types'

defineProps<{ personas: UserPersona[] }>()
</script>

<template>
  <div class="persona-grid">
    <el-card v-for="p in personas" :key="p.competitor" shadow="hover">
      <template #header>{{ p.competitor }} 用户画像</template>
      <p><strong>目标人群</strong></p>
      <el-tag v-for="s in p.segments" :key="s" class="tag">{{ s }}</el-tag>
      <p class="section"><strong>痛点</strong></p>
      <ul>
        <li v-for="x in p.painPoints" :key="x">{{ x }}</li>
      </ul>
      <p class="section"><strong>使用场景</strong></p>
      <ul>
        <li v-for="x in p.useCases" :key="x">{{ x }}</li>
      </ul>
    </el-card>
  </div>
</template>

<style scoped>
.persona-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}
.tag {
  margin: 0 6px 6px 0;
}
.section {
  margin-top: 12px;
}
ul {
  margin: 4px 0 0;
  padding-left: 18px;
  font-size: 13px;
}
</style>
