import { describe, it, expect } from 'vitest'
import { ref, nextTick } from 'vue'
import { useTablePaging } from './useTablePaging'

interface Row {
  name: string
  age: number
  active: boolean
}

const data = [
  { name: 'charlie', age: 30, active: true },
  { name: 'alice', age: 25, active: false },
  { name: 'bob', age: 35, active: true },
  { name: 'dave', age: 20, active: false },
]

describe('useTablePaging', () => {
  it('排序：升序 / 降序 / 清除', async () => {
    const source = ref<Row[]>([...data])
    const { paged, onSortChange } = useTablePaging(source, 20)

    onSortChange({ prop: 'age', order: 'ascending' })
    await nextTick()
    expect(paged.value.map((r) => r.age)).toEqual([20, 25, 30, 35])

    onSortChange({ prop: 'age', order: 'descending' })
    await nextTick()
    expect(paged.value.map((r) => r.age)).toEqual([35, 30, 25, 20])

    onSortChange({ prop: 'age', order: null })
    await nextTick()
    expect(paged.value.map((r) => r.name)).toEqual(['charlie', 'alice', 'bob', 'dave'])
  })

  it('布尔排序：false(0) 在 true(1) 前', async () => {
    const source = ref<Row[]>([...data])
    const { paged, onSortChange } = useTablePaging(source, 20)

    onSortChange({ prop: 'active', order: 'ascending' })
    await nextTick()
    expect(paged.value.map((r) => r.active)).toEqual([false, false, true, true])
  })

  it('字符串排序走 localeCompare', async () => {
    const source = ref<Row[]>([...data])
    const { paged, onSortChange } = useTablePaging(source, 20)

    onSortChange({ prop: 'name', order: 'ascending' })
    await nextTick()
    expect(paged.value.map((r) => r.name)).toEqual(['alice', 'bob', 'charlie', 'dave'])
  })

  it('分页：切片与翻页', async () => {
    const source = ref<Row[]>([...data])
    const { paged, currentPage, pageSize, total } = useTablePaging(source, 2)

    expect(total.value).toBe(4)
    expect(pageSize.value).toBe(2)
    expect(paged.value.length).toBe(2)
    expect(paged.value[0].name).toBe('charlie')

    currentPage.value = 2
    await nextTick()
    expect(paged.value[0].name).toBe('bob')
  })

  it('派生列 getter 排序', async () => {
    const source = ref<Row[]>([...data])
    // 派生列：name 长度
    const { paged, onSortChange } = useTablePaging(source, 20, {
      sortKey: (row) => row.name.length,
    })
    // name 长度：bob(3) < dave(4) < alice(5) < charlie(7)
    onSortChange({ prop: '__derived__', order: 'ascending' })
    await nextTick()
    expect(paged.value.map((r) => r.name)).toEqual(['bob', 'dave', 'alice', 'charlie'])
  })
})
