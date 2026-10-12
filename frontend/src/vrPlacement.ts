// VR(Meta Quest 等の WebXR)で見始めたときに、モデルをどこへどの大きさで
// 置くか。WebXR の 'local-floor' 空間では、原点がセッション開始時に立っていた
// 位置の床で、+Y が上、正面が -Z。単位はメートル(Blender・glTF と同じ)。

export type Size3 = { x: number; y: number; z: number };

// scale: モデルに掛ける倍率(1 = 原寸)
// lift: モデルの底面を床から持ち上げる高さ(m)
export type VrPlacement = { scale: number; lift: number };

// 目の前からモデルの手前の縁までの距離(m)。手を伸ばせば掴める程度
export const VR_REACH = 0.6;

// 常に原寸で床に置く。VR で見る価値は実寸の把握にあるので、大きさで倍率を
// 変えない(変えると「いま原寸か」が VR の中から分からなくなる)。
// size は今は使わないが、大きさで配置を変えたくなったときの入口として残す
export function vrPlacementFor(_size: Size3): VrPlacement {
  return { scale: 1, lift: 0 };
}
