<template>
  <div class="flex items-center gap-4">
    <label class="flex items-center gap-2 cursor-pointer">
      <input type="radio" :name="name" :value="1" :checked="modelValue === 1" @change="$emit('update:modelValue', 1)" class="accent-green-600" />
      <span class="text-sm text-green-700 font-medium">Enabled</span>
    </label>
    <label class="flex items-center gap-2 cursor-pointer">
      <input type="radio" :name="name" :value="2" :checked="modelValue === 2" @change="$emit('update:modelValue', 2)" class="accent-gray-500" />
      <span class="text-sm text-gray-600 font-medium">Disabled</span>
    </label>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: number
  name?: string
}>()

defineEmits<{
  'update:modelValue': [value: number]
}>()
</script>
