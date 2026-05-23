<script setup lang="ts">
import type { PricingInfo } from '@/types'

defineProps<{ pricing: PricingInfo[] }>()
</script>

<template>
  <div class="pricing-row">
    <el-card
      v-for="p in pricing"
      :key="p.competitor"
      class="pricing-card"
      shadow="hover"
    >
      <template #header>
        <strong>{{ p.competitor }}</strong>
      </template>
      <div v-for="tier in p.tiers" :key="tier.name" class="tier">
        <div class="tier-head">
          <span>{{ tier.name }}</span>
          <el-tag type="warning">{{ tier.price }}</el-tag>
        </div>
        <ul>
          <li v-for="f in tier.features" :key="f">{{ f }}</li>
        </ul>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.pricing-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}
.tier {
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-subtle);
}
.tier:last-child {
  border-bottom: none;
}
.tier-head {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}
.tier ul {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  color: var(--text-secondary);
}
</style>
