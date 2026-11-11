import type { Modifier } from '@dnd-kit/core'

/**
 * 拖拽时锁定 X 轴，只允许上下移动。
 * 避免手机端拖动时左右滑动把行拖偏、或带动页面横向滚动。
 */
export const restrictToVerticalAxis: Modifier = ({ transform }) => ({
  ...transform,
  x: 0,
})
