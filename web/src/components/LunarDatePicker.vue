<template>
  <Teleport to="body">
    <!-- 手机端：遮罩，点击等同取消（还原打开时的日期） -->
    <div v-if="isMobile" class="lunar-sheet-backdrop" @click="cancelSheet"></div>
    <div
      ref="rootEl"
      role="dialog"
      aria-label="选择日期"
      :class="isMobile ? 'lunar-sheet' : 'lunar-picker'"
      :style="isMobile ? undefined : floatStyle"
    >
      <div v-if="isMobile" class="mx-auto mb-2 h-1 w-10 rounded-full bg-border" aria-hidden="true"></div>
      <div class="flex items-center justify-between px-1 pb-2">
        <div class="tab-group" role="tablist">
          <button type="button" class="tab-btn" :class="{ active: mode === 'solar' }" role="tab" :aria-selected="mode === 'solar'" @click="switchMode('solar')">公历</button>
          <button type="button" class="tab-btn" :class="{ active: mode === 'lunar' }" role="tab" :aria-selected="mode === 'lunar'" @click="switchMode('lunar')">农历</button>
        </div>
        <button type="button" class="today-btn" @click="goToday">今天</button>
      </div>

      <!-- 公历页签：公历月历，格子下标农历/节日 -->
      <template v-if="mode === 'solar'">
        <div class="flex items-center justify-between px-1 pb-2">
          <div class="flex items-center gap-1">
            <button type="button" class="nav-btn" aria-label="上个月" @click="shiftMonth(-1)">
              <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m15 6-6 6 6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
            <span class="min-w-[96px] text-center text-sm font-bold">{{ viewYear }} 年 {{ viewMonth }} 月</span>
            <button type="button" class="nav-btn" aria-label="下个月" @click="shiftMonth(1)">
              <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m9 6 6 6-6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
          </div>
        </div>
        <div class="grid grid-cols-7 border-b border-border/70 pb-1 text-center text-[10px] font-bold text-muted-foreground">
          <span v-for="w in weekdays" :key="w">{{ w }}</span>
        </div>
        <div v-if="loading" class="grid h-[240px] place-items-center text-xs text-muted-foreground">加载农历…</div>
        <div v-else class="grid grid-cols-7 gap-y-0.5 pt-1" @touchstart.passive="onTouchStart" @touchend.passive="onTouchEnd">
          <button
            v-for="(cell, i) in cells"
            :key="i"
            type="button"
            class="day-cell"
            :class="{ dim: !cell.in_month, today: isSolarToday(cell), selected: isSolarSelected(cell) }"
            :title="`${cell.year}-${String(cell.month).padStart(2, '0')}-${String(cell.day).padStart(2, '0')} 农历${(cell.lunar_month < 0 ? '闰' : '') + Math.abs(cell.lunar_month)}月${cell.lunar}${cell.lunar_year_name ? ' ' + cell.lunar_year_name : ''}`"
            @click="pickSolar(cell)"
          >
            <span class="day-num" :class="{ 'festival-num': cell.festival || cell.term }">{{ cell.day }}</span>
            <span class="day-sub" :class="{ 'festival-sub': cell.festival, 'term-sub': !cell.festival && cell.term }">
              {{ cell.festival || cell.term || cell.lunar }}
            </span>
          </button>
        </div>
      </template>

      <!-- 农历页签：直接选农历年/月/日 -->
      <template v-else>
        <div class="flex items-center justify-between px-1 pb-2">
          <div class="flex items-center gap-1">
            <button type="button" class="nav-btn" aria-label="上一个农历月" @click="shiftLunarMonth(-1)">
              <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m15 6-6 6 6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
            <select v-model.number="lunarView.month" class="lunar-select" aria-label="农历月份">
              <option v-for="m in lunarMonthOptions" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
            <select v-model.number="lunarView.year" class="lunar-select !w-[88px]" aria-label="农历年份">
              <option v-for="y in lunarYearOptions" :key="y" :value="y">{{ y }}</option>
            </select>
            <button type="button" class="nav-btn" aria-label="下一个农历月" @click="shiftLunarMonth(1)">
              <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m9 6 6 6-6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
          </div>
          <span class="text-[10px] text-muted-foreground">{{ lunarViewYearName }}</span>
        </div>
        <!-- 农历格子按 7 列顺序排布、与星期无关，且每个日期已带公历标注，
             不放表头以免误导 -->
        <div class="grid grid-cols-7 gap-y-0.5 pt-1" @touchstart.passive="onTouchStart" @touchend.passive="onTouchEnd">
          <button
            v-for="d in lunarViewDays"
            :key="d"
            type="button"
            class="day-cell"
            :class="{ today: isLunarToday(d), selected: isLunarSelected(d) }"
            :title="lunarCellTitle(d)"
            @click="pickLunar(d)"
          >
            <span class="day-num">{{ lunarDayName(d) }}</span>
            <span class="day-sub">{{ lunarCellSub(d) }}</span>
          </button>
        </div>
        <p v-if="lunarView.month < 0" class="px-1 pt-2 text-[10px] leading-4 text-muted-foreground">
          闰月生日在无闰月的年份自动用对应平月。
        </p>
        <p v-if="lunarView.day === 30 && lunarViewDays.length === 29" class="px-1 pt-2 text-[10px] leading-4 text-amber-600 dark:text-amber-400">
          本月只有廿九，选三十将落在除夕当天。
        </p>
      </template>

      <div v-if="withTime" class="mt-3 border-t border-border/70 pt-3">
        <!-- 手机端：时/分两个滚轮下拉，比系统表盘弹窗更好按 -->
        <div v-if="isMobile">
          <p class="mb-1.5 text-xs font-bold text-muted-foreground">时间</p>
          <div class="flex items-center gap-2">
            <select v-model="hourValue" class="time-select" aria-label="小时">
              <option v-for="h in hourOptions" :key="h" :value="h">{{ h }} 时</option>
            </select>
            <span class="text-sm font-bold text-muted-foreground" aria-hidden="true">:</span>
            <select v-model="minuteValue" class="time-select" aria-label="分钟">
              <option v-for="m in minuteOptions" :key="m" :value="m">{{ m }} 分</option>
            </select>
          </div>
        </div>
        <div v-else class="flex items-center gap-2">
          <label class="text-xs font-bold text-muted-foreground">时间</label>
          <input v-model="timeValue" type="time" class="time-input" @input="onTimeChange" @change="onTimeChange" />
        </div>
      </div>

      <!-- 手机端抽屉的确认操作栏：选择实时生效，确定只负责收起 -->
      <div v-if="isMobile" class="mt-3 flex gap-2">
        <button type="button" class="sheet-btn sheet-btn-ghost" @click="cancelSheet">取消</button>
        <button type="button" class="sheet-btn sheet-btn-primary" @click="confirmSheet">确定</button>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { convertLunarDate, getLunarCalendar, type LunarCalendarMonth, type LunarDayInfo } from '../api/reminder'

const props = withDefaults(defineProps<{
  /** 绑定的 Date（本地时区语义），null 表示未选择 */
  modelValue: Date | null
  /** 是否显示时间选择 */
  withTime?: boolean
  /** 触发弹层的锚点元素：弹层 fixed 定位挂到 body，避免被
      overflow:hidden 的祖先（如快速添加卡片）裁剪 */
  anchorEl?: HTMLElement | null
}>(), { withTime: false })

const emit = defineEmits<{
  (e: 'update:modelValue', value: Date | null): void
  (e: 'commit'): void
}>()

// 手机端（<sm）改为底部抽屉交互：遮罩 + 取消/确定；桌面端保持悬浮定位、
// 点击外部关闭。选择过程实时写入 modelValue，确定只负责收起。
const isMobile = ref(false)
let sheetMediaQuery: MediaQueryList | undefined
function syncSheetMode() {
  isMobile.value = !!sheetMediaQuery?.matches
}
// 打开时的初值：取消时还原到这个日期
const initialValue = ref<Date | null>(props.modelValue)

function confirmSheet() {
  emit('commit')
}
function cancelSheet() {
  emit('update:modelValue', initialValue.value)
  emit('commit')
}

const weekdays = ['一', '二', '三', '四', '五', '六', '日']
const loading = ref(false)
const cells = ref<LunarDayInfo[]>([])
const mode = ref<'solar' | 'lunar'>('solar')
const viewYear = ref(0)
const viewMonth = ref(1) // 1-12
const timeValue = ref('09:00')
const rootEl = ref<HTMLElement | null>(null)
const floatStyle = ref<Record<string, string>>({})
let requestSeq = 0

const lunarView = reactive({ year: 0, month: 1, day: 1 })
const lunarYearOptions = computed(() => {
  const current = new Date().getFullYear()
  const from = current - 5
  return Array.from({ length: 11 }, (_, i) => from + i)
})
const lunarMonthOptions = computed(() => {
  const options = []
  for (let m = 1; m <= 12; m++) options.push({ value: m, label: lunarMonthNames[m] })
  // 闰月选项插在平月之后
  const leap = lunarViewLeapMonth.value
  if (leap) options.splice(leap, 0, { value: -leap, label: `闰${lunarMonthNames[leap]}` })
  return options
})
const lunarMonthNames: Record<number, string> = {
  1: '正月', 2: '二月', 3: '三月', 4: '四月', 5: '五月', 6: '六月',
  7: '七月', 8: '八月', 9: '九月', 10: '十月', 11: '冬月', 12: '腊月',
}
// 当前农历年有无闰月、是几月，通过转换接口探测：直接尝试读取月份选项由
// 闰月探测函数维护。
const lunarViewLeapMonth = ref(0)
const lunarViewYearName = ref('')
// 农历页签每个农历日对应的公历月/日：农历日与公历日连续一一对应，
// 只需换算初一的公历日期，其余按天数推算，无需逐日调接口。
const lunarDaySolar = ref<Array<{ m: number; d: number }>>([])
const lunarViewDays = computed(() => {
  // 估算本月天数：用转换接口返回不了天数，这里用已知规律——
  // 探测三十是否存在：把 30 转公历再看其农历日是否为 30。
  if (lunarViewProbeDay30.value === 30) return Array.from({ length: 30 }, (_, i) => i + 1)
  return Array.from({ length: 29 }, (_, i) => i + 1)
})
const lunarViewProbeDay30 = ref(0)
const lunarMonthLabel = computed(() =>
  (lunarView.month < 0 ? '闰' : '') + lunarMonthNames[Math.abs(lunarView.month)]
)

// 探测序号：快速切换农历月时丢弃过期响应，防止公历映射写错月份。
let lunarProbeSeq = 0

async function probeLunarMonth() {
  const seq = ++lunarProbeSeq
  if (!lunarView.year || !lunarView.month) return
  try {
    // 三十转公历后农历日仍是三十 → 本月有三十；否则回退到廿九。
    const res = await convertLunarDate({ lunar_year: lunarView.year, lunar_month: lunarView.month, lunar_day: 30 })
    if (seq !== lunarProbeSeq) return
    lunarViewProbeDay30.value = res.data.data.lunar_day
  } catch {
    if (seq !== lunarProbeSeq) return
    lunarViewProbeDay30.value = 29
  }
  try {
    // 初一转公历得到本月起点，推算整月每个农历日的公历日期。
    const res = await convertLunarDate({ lunar_year: lunarView.year, lunar_month: lunarView.month, lunar_day: 1 })
    if (seq !== lunarProbeSeq) return
    const first = res.data.data
    const base = new Date(first.year, first.month - 1, first.day)
    lunarDaySolar.value = lunarViewDays.value.map((_, i) => {
      const d = new Date(base.getFullYear(), base.getMonth(), base.getDate() + i)
      return { m: d.getMonth() + 1, d: d.getDate() }
    })
  } catch {
    if (seq !== lunarProbeSeq) return
    lunarDaySolar.value = []
  }
  try {
    const res = await convertLunarDate({ lunar_year: lunarView.year, lunar_month: 1, lunar_day: 1 })
    if (seq !== lunarProbeSeq) return
    lunarViewYearName.value = res.data.data.lunar_year_name
  } catch { /* 忽略年份标签失败 */ }
  try {
    // 闰月探测：逐月 30 无所谓，直接询问后端该年闰几月——用转换接口逐个
    // 试探代价太高，改为仅在用户主动选闰月时生效，这里探测常用的 2-9 月。
    const leapRes = await probeLeapMonth(lunarView.year)
    if (seq !== lunarProbeSeq) return
    lunarViewLeapMonth.value = leapRes
  } catch { /* 无闰月 */ }
}

// 农历日的传统叫法：1=初一 … 10=初十、20=二十、21=廿一 … 30=三十。
const lunarDayNames = [
  '初一', '初二', '初三', '初四', '初五', '初六', '初七', '初八', '初九', '初十',
  '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九', '二十',
  '廿一', '廿二', '廿三', '廿四', '廿五', '廿六', '廿七', '廿八', '廿九', '三十',
]
function lunarDayName(day: number) {
  return lunarDayNames[day - 1] || String(day)
}

// 农历格子的副标：优先显示对应公历（如 9/11），换算失败时退回空。
function lunarCellSub(day: number) {
  const solar = lunarDaySolar.value[day - 1]
  return solar ? `${solar.m}/${solar.d}` : ''
}

function lunarCellTitle(day: number) {
  if (day === 30 && lunarViewDays.value.length === 29) return '本月无三十，选择将落在廿九（除夕）'
  const solar = lunarDaySolar.value[day - 1]
  const prefix = solar ? `${solar.m}月${solar.d}日 · ` : ''
  return `${prefix}农历${lunarMonthLabel}第${day}天`
}

// 农历页签也高亮“今天”：公历今天落在当前农历月的第几天。
function isLunarToday(day: number) {
  const now = new Date()
  const solar = lunarDaySolar.value[day - 1]
  return !!solar && solar.m === now.getMonth() + 1 && solar.d === now.getDate()
}

async function probeLeapMonth(year: number): Promise<number> {
  // 农历闰月只可能是 2-11 月（极少 1/12）。从小到大试，命中即返回。
  for (const m of [2, 3, 4, 5, 6, 7, 8, 9, 10, 11]) {
    try {
      const res = await convertLunarDate({ lunar_year: year, lunar_month: -m, lunar_day: 1 })
      const d = res.data.data
      // 转换后农历月保持 -m → 该年确有闰 m 月
      if (d.lunar_month === -m) return m
    } catch { /* 继续探测 */ }
  }
  return 0
}

function switchMode(next: 'solar' | 'lunar') {
  if (mode.value === next) return
  mode.value = next
  if (next === 'lunar') {
    syncLunarFromSolar()
  }
}

// 把当前公历视图（或选中日期）映射到农历视图
function syncLunarFromSolar() {
  const probeDate = props.modelValue ?? new Date(viewYear.value, viewMonth.value - 1, Math.min(viewMonth.value === (new Date()).getMonth() + 1 ? new Date().getDate() : 15, 28))
  convertLunarDate({
    year: probeDate.getFullYear(),
    month: probeDate.getMonth() + 1,
    day: probeDate.getDate(),
  }).then(res => {
    const d = res.data.data
    lunarView.year = d.lunar_year
    lunarView.month = d.lunar_month
    lunarView.day = d.lunar_day
    lunarViewYearName.value = d.lunar_year_name
    void probeLunarMonth()
  }).catch(() => { /* 保持当前视图 */ })
}

function isSolarToday(cell: LunarDayInfo) {
  const now = new Date()
  return cell.year === now.getFullYear() && cell.month === now.getMonth() + 1 && cell.day === now.getDate()
}
function isSolarSelected(cell: LunarDayInfo) {
  if (!props.modelValue) return false
  return cell.year === props.modelValue.getFullYear()
    && cell.month === props.modelValue.getMonth() + 1
    && cell.day === props.modelValue.getDate()
}
function isLunarSelected(day: number) {
  if (!props.modelValue) return false
  // 选中日期转农历后与视图比较
  return lunarSelectedKey.value === `${lunarView.year}:${lunarView.month}:${day}`
}
const lunarSelectedKey = computed(() => {
  if (!props.modelValue) return ''
  const d = props.modelValue
  const cached = lunarKeyCache.get(d.toDateString())
  return cached ?? ''
})
const lunarKeyCache = new Map<string, string>()

watch(() => props.modelValue, value => {
  if (!value) return
  const key = value.toDateString()
  if (!lunarKeyCache.has(key)) {
    convertLunarDate({ year: value.getFullYear(), month: value.getMonth() + 1, day: value.getDate() })
      .then(res => {
        const d = res.data.data
        lunarKeyCache.set(key, `${d.lunar_year}:${d.lunar_month}:${d.lunar_day}`)
      })
      .catch(() => lunarKeyCache.set(key, ''))
  }
  timeValue.value = `${String(value.getHours()).padStart(2, '0')}:${String(value.getMinutes()).padStart(2, '0')}`
}, { immediate: true })

function syncView() {
  const base = props.modelValue ?? new Date()
  viewYear.value = base.getFullYear()
  viewMonth.value = base.getMonth() + 1
  timeValue.value = `${String(base.getHours()).padStart(2, '0')}:${String(base.getMinutes()).padStart(2, '0')}`
}

async function loadMonth() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const res = await getLunarCalendar(viewYear.value, viewMonth.value)
    if (seq !== requestSeq) return
    const data = res.data.data as LunarCalendarMonth
    cells.value = data.days
  } catch {
    if (seq === requestSeq) cells.value = []
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function shiftMonth(delta: number) {
  const next = new Date(viewYear.value, viewMonth.value - 1 + delta, 1)
  viewYear.value = next.getFullYear()
  viewMonth.value = next.getMonth() + 1
}

// 手机端手势：在日历格子上左右轻扫翻月（水平位移明显大于垂直才算，
// 避免和页面滚动冲突）。左扫下个月，右扫上个月。
let gridTouchStart: { x: number; y: number } | null = null
function onTouchStart(e: TouchEvent) {
  const t = e.touches[0]
  gridTouchStart = { x: t.clientX, y: t.clientY }
}
function onTouchEnd(e: TouchEvent) {
  const start = gridTouchStart
  gridTouchStart = null
  if (!start) return
  const t = e.changedTouches[0]
  const dx = t.clientX - start.x
  const dy = t.clientY - start.y
  if (Math.abs(dx) < 60 || Math.abs(dx) < Math.abs(dy) * 1.5) return
  const delta = dx < 0 ? 1 : -1
  if (mode.value === 'solar') shiftMonth(delta)
  else shiftLunarMonth(delta)
}

// 农历月步进：跨年与跳过不存在的闰月选项
function shiftLunarMonth(delta: number) {
  const options = lunarMonthOptions.value.map(o => o.value)
  const idx = options.indexOf(lunarView.month)
  if (idx === -1) {
    lunarView.month = options[0]
    return
  }
  let nextIdx = idx + delta
  if (nextIdx < 0) {
    // 上一年最后一个选项
    lunarView.year -= 1
    lunarView.month = options[options.length - 1]
    void probeLunarMonth()
    return
  }
  if (nextIdx >= options.length) {
    lunarView.year += 1
    lunarView.month = options[0]
    void probeLunarMonth()
    return
  }
  lunarView.month = options[nextIdx]
  void probeLunarMonth()
}

function goToday() {
  const now = new Date()
  viewYear.value = now.getFullYear()
  viewMonth.value = now.getMonth() + 1
  mode.value = 'solar'
  pickSolar({ year: now.getFullYear(), month: now.getMonth() + 1, day: now.getDate() } as LunarDayInfo)
  // 手机端抽屉保持打开，让用户按“确定”收起
  if (!isMobile.value) emit('commit')
}

function pickSolar(cell: LunarDayInfo) {
  const [h, m] = timeValue.value.split(':').map(Number)
  const picked = new Date(cell.year, cell.month - 1, cell.day, h || 0, m || 0, 0, 0)
  emit('update:modelValue', picked)
}

function pickLunar(day: number) {
  lunarView.day = day
  const [h, m] = timeValue.value.split(':').map(Number)
  convertLunarDate({ lunar_year: lunarView.year, lunar_month: lunarView.month, lunar_day: day })
    .then(res => {
      const d = res.data.data
      const picked = new Date(d.year, d.month - 1, d.day, h || 0, m || 0, 0, 0)
      emit('update:modelValue', picked)
      // 手机端抽屉保持打开，让用户按“确定”收起
      if (!isMobile.value) emit('commit')
    })
    .catch(() => { /* 转换失败不更新 */ })
}

// 只改时间：合并进已选日期；还没选日期时默认当天，改完即选中今天。
// 弹层保持打开便于继续调整。（此前这里只关闭弹层、不写回时间，导致“时间改了却不生效”。）
function onTimeChange() {
  const [h, m] = timeValue.value.split(':').map(Number)
  const base = props.modelValue ?? new Date()
  emit('update:modelValue', new Date(base.getFullYear(), base.getMonth(), base.getDate(), h || 0, m || 0, 0, 0))
}

// 手机端时/分滚轮：与 timeValue 双向同步，变更立即生效。
const hourOptions = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
const minuteOptions = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'))
const hourValue = computed({
  get: () => timeValue.value.split(':')[0] || '09',
  set: (h: string) => setTimeParts(h, timeValue.value.split(':')[1] || '00'),
})
const minuteValue = computed({
  get: () => timeValue.value.split(':')[1] || '00',
  set: (m: string) => setTimeParts(timeValue.value.split(':')[0] || '09', m),
})
function setTimeParts(h: string, m: string) {
  timeValue.value = `${h}:${m}`
  onTimeChange()
}

const anchor = computed(() => props.modelValue ?? new Date())

// 以锚点元素计算 fixed 位置：默认在按钮下方左对齐；底部放不下时
// 上翻到按钮上方；左右贴边时向内收。挂载与视口变化时都重新计算。
function place() {
  if (isMobile.value) return // 抽屉由 CSS 固定在底部，无需锚点定位
  const el = rootEl.value
  const anchorNode = props.anchorEl
  if (!el) return
  if (!anchorNode) {
    floatStyle.value = { position: 'fixed', top: '30vh', left: '50%', transform: 'translateX(-50%)' }
    return
  }
  const a = anchorNode.getBoundingClientRect()
  const w = el.offsetWidth || 320
  const h = el.offsetHeight || 380
  const margin = 8
  let top = a.bottom + margin
  if (top + h > innerHeight - margin && a.top - margin - h > margin) {
    top = a.top - margin - h
  }
  top = Math.max(margin, Math.min(top, Math.max(margin, innerHeight - h - margin)))
  let left = a.left
  if (left + w > innerWidth - margin) {
    left = Math.max(margin, innerWidth - margin - w)
  }
  floatStyle.value = { position: 'fixed', top: `${Math.round(top)}px`, left: `${Math.round(left)}px`, transform: 'none' }
  // 弹层宽度可能超过窄视口：先落位再量实际宽度，超宽时固定 320px 上限并二次贴边。
  if (el.offsetWidth > innerWidth - margin * 2) {
    floatStyle.value = { ...floatStyle.value, width: `${innerWidth - margin * 2}px` }
  }
}

// 点击弹层外部关闭：仅桌面端（手机端由遮罩处理，且点遮罩=取消还原）。
function onDocumentPointerDown(event: PointerEvent) {
  if (isMobile.value) return
  const target = event.target as Node
  if (rootEl.value?.contains(target)) return
  if (props.anchorEl?.contains(target)) return
  emit('commit')
}

onMounted(() => {
  sheetMediaQuery = window.matchMedia('(max-width: 639px)')
  syncSheetMode()
  sheetMediaQuery.addEventListener('change', syncSheetMode)
  // 打开时还没有日期：默认选中今天（时间用当前时分），可直接按“确定”确认。
  // “取消”仍会还原为打开前的状态（不选日期）。
  if (!props.modelValue) {
    const [h, m] = timeValue.value.split(':').map(Number)
    const now = new Date()
    emit('update:modelValue', new Date(now.getFullYear(), now.getMonth(), now.getDate(), h || 0, m || 0, 0, 0))
  }
  place()
  document.addEventListener('pointerdown', onDocumentPointerDown, true)
  window.addEventListener('resize', place)
  window.addEventListener('scroll', place, true)
})
onBeforeUnmount(() => {
  sheetMediaQuery?.removeEventListener('change', syncSheetMode)
  document.removeEventListener('pointerdown', onDocumentPointerDown, true)
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', place, true)
})

defineExpose({ place })

watch([viewYear, viewMonth], loadMonth, { immediate: false })
watch([lunarView.year, lunarView.month], () => void probeLunarMonth())

syncView()
void loadMonth()
// 等首轮渲染完成后再定位，否则 offsetHeight 还是 0。
void Promise.resolve().then(place)
</script>

<style scoped>
.lunar-picker { @apply z-[80] w-[320px] max-w-full rounded-2xl border border-border bg-surface p-3 shadow-lg backdrop-blur; }

/* 手机端底部抽屉：固定贴底、圆角顶、内部可滚动，滑入动画呈现 */
.lunar-sheet {
  @apply fixed inset-x-0 bottom-0 z-[80] max-h-[86dvh] overflow-y-auto rounded-t-3xl border-t border-border bg-surface p-4 shadow-[0_-18px_50px_-20px_rgba(15,23,42,.45)] pb-[calc(1rem+env(safe-area-inset-bottom))];
  animation: lunar-sheet-up 0.26s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.lunar-sheet-backdrop { @apply fixed inset-0 z-[79] bg-slate-950/40; animation: lunar-backdrop-in 0.2s ease; }
.sheet-btn { @apply h-11 flex-1 rounded-xl text-sm font-bold transition; }
.sheet-btn-ghost { @apply border border-border bg-surface text-foreground hover:bg-muted; }
.sheet-btn-primary { @apply bg-brand-gradient text-white shadow-glow hover:brightness-105; }
@keyframes lunar-sheet-up { from { transform: translateY(100%); } }
@keyframes lunar-backdrop-in { from { opacity: 0; } }
.tab-group { @apply flex rounded-xl bg-muted p-0.5; }
.tab-btn { @apply rounded-lg px-3 py-2 text-xs font-bold text-muted-foreground transition; }
.tab-btn.active { @apply bg-surface text-foreground shadow-sm; }
.nav-btn { @apply flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition hover:bg-muted hover:text-foreground; }
.today-btn { @apply rounded-lg border border-border px-3 py-2 text-xs font-bold text-muted-foreground transition hover:border-brand-500/40 hover:text-brand-600; }
.lunar-select { @apply h-9 rounded-lg border border-border bg-surface px-2 text-[16px] sm:text-xs font-bold text-foreground outline-none focus:border-brand-500/50; }
.day-cell { @apply flex h-[46px] flex-col items-center justify-center rounded-xl px-0.5 py-1 transition hover:bg-muted; }
.day-cell.dim { @apply opacity-35; }
.day-cell.today .day-num { @apply text-brand-600 dark:text-brand-300; }
.day-cell.selected { @apply bg-brand-500/10 ring-2 ring-brand-500/40; }
.day-num { @apply text-[13px] font-bold leading-none text-foreground; }
.festival-num { @apply text-rose-500; }
.day-sub { @apply mt-1 max-w-full truncate text-[9px] leading-none text-muted-foreground; }
.festival-sub { @apply font-bold text-rose-500; }
.term-sub { @apply font-bold text-emerald-600 dark:text-emerald-400; }
.time-input { @apply h-10 flex-1 rounded-xl border border-border bg-surface px-3 text-[16px] sm:text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
.time-select { @apply h-10 w-[104px] rounded-xl border border-border bg-surface px-2 text-[16px] font-medium text-foreground outline-none focus:border-brand-500/50; }
</style>
