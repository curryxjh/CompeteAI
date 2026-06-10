<script setup lang="ts">
defineProps<{ skills: string[]; tools: string[] }>()
</script>

<template>
  <div class="skills">
    <p><strong>技能</strong></p>
    <el-tag v-for="s in skills" :key="s" size="small" class="tag">{{ s }}</el-tag>
    <p class="tools-title"><strong>Tools</strong></p>
    <el-tag v-for="t in tools" :key="t" size="small" type="success" class="tag">{{ t }}</el-tag>
  </div>
</template>

<style scoped>
.skills p {
  margin: 0 0 6px;
  font-size: 12px;
}
.tools-title {
  margin-top: 10px !important;
}
.tag {
  margin: 0 4px 4px 0;
}
</style>
