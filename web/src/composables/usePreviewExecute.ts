import { ref, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { extractErrorMessage } from '@/api/client'
import { i18n } from '@/i18n'

const t = i18n.global.t

/** 预览过期倒计时（后端 TTL 默认 5 分钟） */
const PREVIEW_TTL_SECONDS = 300

/**
 * 预览→执行通用流程：
 * - preview(fn)：创建预览，成功后开始 TTL 倒计时
 * - execute(fn)：以 preview_id 执行；预览过期（400）时标记 expired 引导重新预览
 * - reset()：清空状态
 */
export function usePreviewExecute<TPreview>() {
  const previewData = ref<TPreview | null>(null)
  const previewLoading = ref(false)
  const executing = ref(false)
  const countdown = ref(0)
  const expired = ref(false)

  let timer: ReturnType<typeof setInterval> | null = null

  function stopTimer(): void {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
  }

  function startTimer(): void {
    stopTimer()
    countdown.value = PREVIEW_TTL_SECONDS
    timer = setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0) {
        stopTimer()
        expired.value = true
      }
    }, 1000)
  }

  /** 创建预览；成功返回 true */
  async function preview(fn: () => Promise<TPreview>): Promise<boolean> {
    previewLoading.value = true
    try {
      previewData.value = await fn()
      expired.value = false
      startTimer()
      return true
    } catch (err) {
      ElMessage.error(extractErrorMessage(err, t('preview.title')))
      return false
    } finally {
      previewLoading.value = false
    }
  }

  /**
   * 执行预览；返回 { ok, result } 或 { ok: false, error }。
   * 预览过期（400 "预览已过期或不存在"）时置 expired = true。
   */
  async function execute<R>(
    fn: (previewId: string) => Promise<R>,
  ): Promise<{ ok: boolean; result?: R; error?: string }> {
    const data = previewData.value as { preview_id?: string } | null
    if (!data?.preview_id) {
      return { ok: false, error: t('preview.expired') }
    }
    executing.value = true
    try {
      const result = await fn(data.preview_id)
      stopTimer()
      return { ok: true, result }
    } catch (err) {
      const msg = extractErrorMessage(err, t('common.execFailed'))
      // 契约：预览过期或不存在 → 需重新预览
      if (msg.includes('预览已过期') || msg.includes('预览类型不匹配')) {
        expired.value = true
        stopTimer()
      }
      // 执行失败（500）时预览保留，可原 preview_id 重试
      return { ok: false, error: msg }
    } finally {
      executing.value = false
    }
  }

  function reset(): void {
    stopTimer()
    previewData.value = null
    expired.value = false
    executing.value = false
    countdown.value = 0
  }

  onBeforeUnmount(stopTimer)

  return { previewData, previewLoading, executing, countdown, expired, preview, execute, reset }
}
