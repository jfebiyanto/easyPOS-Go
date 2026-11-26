/** Logika murni kunci idle (dipisah agar bisa dites tanpa DOM). */
export const HEARTBEAT_MS = 30_000

export function isIdle(now: number, lastActivity: number, idleMinutes: number): boolean {
  return now - lastActivity >= idleMinutes * 60_000
}

/** Heartbeat hanya dikirim bila ada aktivitas sejak heartbeat terakhir. */
export function needsHeartbeat(now: number, lastActivity: number, lastBeat: number): boolean {
  return lastActivity > lastBeat && now - lastBeat >= HEARTBEAT_MS
}
