/**
 * 背景设置共享状态。
 * 状态由 Layout 持有（localStorage 持久化），通过 <Outlet context> 传入各页面，
 * 这样「背景设置」页和布局里的渲染逻辑用的是同一份数据。
 */
export interface BgContext {
  /** -1 = 无背景，-2 = 自定义图片，0..4 = 预设图下标 */
  bgIndex: number
  /** 自定义图片地址（bgIndex === -2 时有效） */
  bgCustomUrl: string
  selectBg: (idx: number) => void
  selectCustom: (url: string) => void
}
