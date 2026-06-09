<script setup lang="ts">
const props = withDefaults(defineProps<{
  playing?: boolean
  speed?: number
  stepIndex?: number
  maxSteps?: number
}>(), {
  playing: false,
  speed: 1,
  stepIndex: 0,
  maxSteps: 0,
})

defineEmits<{
  step: [index: number]
  play: []
  pause: []
  reset: []
  speed: [value: number]
  export: []
}>()
</script>

<template>
  <div class="replay-bar panel font-mono">
    <button type="button" class="btn-ghost" @click="props.playing ? $emit('pause') : $emit('play')">
      {{ props.playing ? 'pause' : 'replay' }}
    </button>
    <button type="button" class="btn-ghost" @click="$emit('step', props.stepIndex + 1)">step +1</button>
    <button type="button" class="btn-ghost" @click="$emit('reset')">reset</button>
    <span class="sep">|</span>
    <el-radio-group :model-value="props.speed" size="small" @update:model-value="(value: string | number) => $emit('speed', Number(value))">
      <el-radio-button :value="1">1x</el-radio-button>
      <el-radio-button :value="2">2x</el-radio-button>
      <el-radio-button :value="4">4x</el-radio-button>
    </el-radio-group>
    <span class="progress">step {{ Math.min(props.stepIndex, props.maxSteps) }} / {{ props.maxSteps }}</span>
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

.progress {
  color: var(--text-muted);
}
</style>
