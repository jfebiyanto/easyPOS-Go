import { describe, expect, it } from 'vitest'
import { HEARTBEAT_MS, isIdle, needsHeartbeat } from './idle'

describe('isIdle', () => {
  it('belum idle di bawah batas, idle tepat di batas', () => {
    expect(isIdle(14 * 60_000, 0, 15)).toBe(false)
    expect(isIdle(15 * 60_000, 0, 15)).toBe(true)
  })
})

describe('needsHeartbeat', () => {
  it('tanpa aktivitas baru tidak mengirim', () => {
    expect(needsHeartbeat(10 * HEARTBEAT_MS, 5, 5)).toBe(false)
  })
  it('ada aktivitas tetapi belum 30 dtk dari heartbeat terakhir: tahan', () => {
    expect(needsHeartbeat(HEARTBEAT_MS - 1, 1000, 0)).toBe(false)
  })
  it('ada aktivitas dan sudah 30 dtk: kirim', () => {
    expect(needsHeartbeat(HEARTBEAT_MS, 1000, 0)).toBe(true)
  })
})
