/** Parse UTC timestamp and format as CST (UTC+8). */
export function toCSTString(ts: string): string {
  const hour = parseInt(ts.slice(11, 13), 10)
  const min = ts.slice(14, 16)
  const cstHour = (hour + 8) % 24
  return `${String(cstHour).padStart(2, '0')}:${min}`
}

/** Full CST datetime, e.g. "6/8 01:05". */
export function toCSTFull(ts: string): string {
  const month = parseInt(ts.slice(5, 7), 10)
  const day = parseInt(ts.slice(8, 10), 10)
  const hour = parseInt(ts.slice(11, 13), 10)
  const min = ts.slice(14, 16)
  let cstHour = hour + 8
  let cstDay = day
  let cstMonth = month
  if (cstHour >= 24) {
    cstHour -= 24
    cstDay += 1
  }
  return `${cstMonth}/${cstDay} ${String(cstHour).padStart(2, '0')}:${min}`
}
