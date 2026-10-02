/**
 * 作品瀑布流里一格图片的高度（rpx）。
 * 不按 ratio × 列宽 算：那样高度跟着屏幕宽度变，比例大时一格能到半屏。
 * ratio 只决定落在哪一档，档位对齐原型（170–230px），最高 440rpx。
 */
const TIERS: [maxRatio: number, height: number][] = [
  [1.1, 320],  // 方图
  [1.29, 360], // 4:5
  [1.41, 400], // 3:4
]
const TALLEST = 440 // 2:3 及更高

export function workImageHeight(ratio: number): number {
  return TIERS.find(([max]) => ratio <= max)?.[1] ?? TALLEST
}
