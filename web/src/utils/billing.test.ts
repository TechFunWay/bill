import { describe, expect, it } from 'vitest'
import { centsToMoney, isSplitValid, moneyToCents, splitByWeights, splitEqually, splitExactly } from './billing'

describe('money conversion', () => {
  it('converts decimal money without floating point leakage', () => {
    expect(moneyToCents('12.345')).toBe(1235)
    expect(moneyToCents('not money')).toBe(0)
    expect(centsToMoney(1235)).toBe('12.35')
  })
})

describe('shared bill splits', () => {
  const ids = [11, 22, 33]

  it('splits equally and assigns remainder cents predictably', () => {
    expect(splitEqually(100, ids)).toEqual([
      { user_id: 11, amount_cents: 34 }, { user_id: 22, amount_cents: 33 }, { user_id: 33, amount_cents: 33 },
    ])
  })

  it('uses exact entered amounts', () => {
    const values = { 11: '1.20', 22: '2.30', 33: '0.50' }
    expect(splitExactly(ids, values)).toEqual([
      { user_id: 11, amount_cents: 120 }, { user_id: 22, amount_cents: 230 }, { user_id: 33, amount_cents: 50 },
    ])
    expect(isSplitValid('exact', 400, ids, values)).toBe(true)
    expect(isSplitValid('exact', 1, ids, { 11: '0.005', 22: '0.005', 33: '0' })).toBe(false)
    expect(isSplitValid('exact', 2, ids, { 11: '0.005', 22: '0.005', 33: '0' })).toBe(true)
  })

  it('apportions percentage weights and preserves every cent', () => {
    const result = splitByWeights(101, ids, { 11: 50, 22: 30, 33: 20 })
    expect(result).toEqual([
      { user_id: 11, amount_cents: 50 }, { user_id: 22, amount_cents: 30 }, { user_id: 33, amount_cents: 21 },
    ])
    expect(isSplitValid('percent', 101, ids, { 11: 50, 22: 30, 33: 20 })).toBe(true)
  })

  it('apportions share weights and rejects zero shares', () => {
    expect(splitByWeights(1000, ids, { 11: 1, 22: 2, 33: 1 })).toEqual([
      { user_id: 11, amount_cents: 250 }, { user_id: 22, amount_cents: 500 }, { user_id: 33, amount_cents: 250 },
    ])
    expect(isSplitValid('shares', 1000, ids, { 11: 0, 22: 0, 33: 0 })).toBe(false)
  })

  it('rejects negative and non-finite split values consistently', () => {
    expect(isSplitValid('percent', 100, ids, { 11: -10, 22: 110, 33: 0 })).toBe(false)
    expect(isSplitValid('shares', 100, ids, { 11: -1, 22: 2, 33: 0 })).toBe(false)
    expect(isSplitValid('shares', 100, ids, { 11: Infinity, 22: 1, 33: 1 })).toBe(false)
    expect(isSplitValid('percent', 100, ids, { 11: 'not-a-number', 22: 100, 33: 0 })).toBe(false)
    expect(splitByWeights(100, ids, { 11: -1, 22: 2, 33: 0 })).toEqual([])
    expect(splitExactly(ids, { 11: '-1', 22: '2', 33: '0' })).toEqual([])
  })
})
