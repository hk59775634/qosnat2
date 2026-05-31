import { describe, expect, it } from 'vitest'
import { copyText } from './clipboard'

describe('copyText', () => {
  it('returns false for empty input', async () => {
    expect(await copyText('')).toBe(false)
    expect(await copyText(null)).toBe(false)
  })
})
