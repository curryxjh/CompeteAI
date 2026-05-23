<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  step: [index: number]
  play: []
  pause: []
  export: []
}>()

const playing = ref(false)
const speed = ref(1)
const stepIndex = ref(0)

function togglePlay() {
  playing.value = !playing.value
  if (playing.value) emit('play')
  else emit('pause')
}

function stepForward() {
  stepIndex.value += 1
  emit('step', stepIndex.value)
}

function reset() {
  stepIndex.value = 0
  playing.value = false
  emit('step', 0)
}
</script>

<template>
  <div class="replay-bar panel font-mono">
    <button type="button" class="btn-ghost" @click="togglePlay">
      {{ playing ? 'pause' : 'replay' }}
    </button>
    <button type="button" class="btn-ghost" @click="stepForward">step +1</button>
    <button type="button" class="btn-ghost" @click="reset">reset</button>
    <span class="sep">|</span>
    <el-radio-group v-model="speed" size="small">
      <el-radio-button :value="1">1x</el-radio-button>
      <el-radio-button :value="2">2x</el-radio-button>
      <el-radio-button :value="4">4x</el-radio-button>
    </el-radio-group>
    <button type="button" class="btn-ghost" @click="$emit('export')">export json</button>
  </div>
</template>

<style scoped>
.replay-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  padding: 10px 14px;
  font-size: 12px;
}
.sep {
  color: var(--text-muted);
  opacity: 0.5;
}
</style>
