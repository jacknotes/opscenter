import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'
import { useDashboardFullscreen } from '../useDashboardFullscreen'

describe('useDashboardFullscreen', () => {
  let composable

  beforeEach(() => {
    composable = useDashboardFullscreen()
  })

  afterEach(() => {
    composable.cleanup()
  })

  describe('toggleFullscreen', () => {
    it('sets fullscreenChart when toggling a chart that is not fullscreen', () => {
      composable.toggleFullscreen('loginStats')
      expect(composable.fullscreenChart.value).toBe('loginStats')
    })

    it('clears fullscreenChart when toggling the same chart again', () => {
      composable.toggleFullscreen('loginStats')
      composable.toggleFullscreen('loginStats')
      expect(composable.fullscreenChart.value).toBeNull()
    })

    it('switches to a different chart when another is already fullscreen', () => {
      composable.toggleFullscreen('loginStats')
      composable.toggleFullscreen('deployTrend')
      expect(composable.fullscreenChart.value).toBe('deployTrend')
    })
  })

  describe('ESC key exits fullscreen', () => {
    it('exits fullscreen when ESC is pressed', () => {
      composable.toggleFullscreen('loginStats')
      expect(composable.fullscreenChart.value).toBe('loginStats')

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
      expect(composable.fullscreenChart.value).toBeNull()
    })

    it('does nothing when ESC is pressed and no chart is fullscreen', () => {
      expect(composable.fullscreenChart.value).toBeNull()

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
      expect(composable.fullscreenChart.value).toBeNull()
    })

    it('does not exit fullscreen when ESC is pressed inside an input element', () => {
      composable.toggleFullscreen('loginStats')

      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))

      expect(composable.fullscreenChart.value).toBe('loginStats')
      document.body.removeChild(input)
    })

    it('does not exit fullscreen when ESC is pressed inside a textarea', () => {
      composable.toggleFullscreen('loginStats')

      const textarea = document.createElement('textarea')
      document.body.appendChild(textarea)
      textarea.focus()
      textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))

      expect(composable.fullscreenChart.value).toBe('loginStats')
      document.body.removeChild(textarea)
    })
  })

  describe('scroll to chart on exit', () => {
    it('scrolls the chart card into view when exiting fullscreen', async () => {
      const card = document.createElement('div')
      card.setAttribute('data-chart', 'loginStats')
      card.scrollIntoView = vi.fn()
      document.body.appendChild(card)

      composable.toggleFullscreen('loginStats')
      await composable.toggleFullscreen('loginStats') // exit fullscreen

      expect(card.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
      document.body.removeChild(card)
    })

    it('does not scroll when entering fullscreen', async () => {
      const card = document.createElement('div')
      card.setAttribute('data-chart', 'loginStats')
      card.scrollIntoView = vi.fn()
      document.body.appendChild(card)

      await composable.toggleFullscreen('loginStats') // enter fullscreen

      expect(card.scrollIntoView).not.toHaveBeenCalled()
      document.body.removeChild(card)
    })

    it('scrolls to chart when ESC exits fullscreen', async () => {
      const card = document.createElement('div')
      card.setAttribute('data-chart', 'deployTrend')
      card.scrollIntoView = vi.fn()
      document.body.appendChild(card)

      composable.toggleFullscreen('deployTrend')
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
      await nextTick() // ESC handler uses toggleFullscreen which awaits nextTick

      expect(card.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
      expect(composable.fullscreenChart.value).toBeNull()
      document.body.removeChild(card)
    })

    it('waits for Vue re-render before scrolling', async () => {
      const card = document.createElement('div')
      card.setAttribute('data-chart', 'loginStats')
      card.scrollIntoView = vi.fn()
      document.body.appendChild(card)

      composable.toggleFullscreen('loginStats')
      const exitPromise = composable.toggleFullscreen('loginStats')

      // scrollIntoView should NOT have been called yet (before nextTick)
      expect(card.scrollIntoView).not.toHaveBeenCalled()

      await exitPromise

      // scrollIntoView should have been called after nextTick
      expect(card.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
      document.body.removeChild(card)
    })
  })

  describe('getFullscreenCardStyle', () => {
    it('returns fullscreen style for the active chart', () => {
      composable.toggleFullscreen('loginStats')
      const style = composable.getFullscreenCardStyle('loginStats')
      expect(style).toEqual({ flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' })
    })

    it('returns empty style for inactive chart', () => {
      composable.toggleFullscreen('loginStats')
      const style = composable.getFullscreenCardStyle('deployTrend')
      expect(style).toEqual({})
    })

    it('returns empty style when no chart is fullscreen', () => {
      const style = composable.getFullscreenCardStyle('loginStats')
      expect(style).toEqual({})
    })
  })

  describe('cleanup', () => {
    it('removes the keydown listener after cleanup', () => {
      const spy = vi.spyOn(document, 'removeEventListener')
      composable.cleanup()
      expect(spy).toHaveBeenCalledWith('keydown', expect.any(Function))
      spy.mockRestore()
    })
  })
})
