import { ref } from 'vue'
import { ElMessage } from 'element-plus'

/**
 * 预览→执行流程组合式函数
 * @param {Record<string, Function>} executeFnMap - action 到执行函数的映射
 * @param {Function} loadDataFn - 执行成功后刷新数据的函数
 * @param {object} opts
 * @param {Function} opts.onSuccess - 执行成功后的额外回调
 * @param {Function} opts.onOutput - 自定义输出处理 (res) => string
 * @returns {{ previewVisible, previewData, previewId, executing, currentAction, openPreview, executePreview }}
 */
export function usePreviewExecute(executeFnMap, loadDataFn, opts = {}) {
  const previewVisible = ref(false)
  const previewData = ref(null)
  const previewId = ref('')
  const executing = ref(false)
  const currentAction = ref('')

  function openPreview(res, action) {
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewVisible.value = true
  }

  async function executePreview() {
    const executeFn = executeFnMap[currentAction.value]
    if (!executeFn) {
      ElMessage.error('未知的操作类型')
      return
    }

    executing.value = true
    try {
      const res = await executeFn({ preview_id: previewId.value })
      if (opts.onOutput) {
        opts.onOutput(res)
      }
      ElMessage.success('执行成功')
      previewVisible.value = false
      if (opts.onSuccess) opts.onSuccess(res)
      try {
        await loadDataFn()
      } catch (e) {
        // loadData 已经处理了错误提示
      }
    } catch (e) {
      ElMessage.error(e.response?.data?.error || '执行失败')
      throw e
    } finally {
      executing.value = false
    }
  }

  return {
    previewVisible,
    previewData,
    previewId,
    executing,
    currentAction,
    openPreview,
    executePreview,
  }
}
