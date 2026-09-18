/**
 * Field geometry constants shared by the renderer (field3d.ts) and the
 * animation functions (field3d_anim.ts). Living in their own module keeps the
 * two from forming a runtime import cycle — the anim functions need ZONE_COORDS
 * while field3d.ts imports those same functions.
 */

// Card dimensions in 3D world units
export const CARD_WIDTH = 1.4;
export const CARD_HEIGHT = 2.0;
export const CARD_DEPTH = 0.03;

// Field Zone coordinate definitions
// 区域坐标 = YGOPro 原版布局（source/ygopro/gframe/materials.cpp 的
// vField* 四边形，权威依据），换算 world = ((qx-4)*2, qy*2)：
// 原版地垫四边形是 x -1..9 / y -4..4 的 10×8，中心 (4,0) 平移到世界原点，
// 整体 ×2 使地垫铺满 20×16 的可视范围。卡片尺寸对应原版 vCardFront
// 0.7×1.0 ×2 = CARD_WIDTH×CARD_HEIGHT。mzone 7 槽（5 主怪区 + 两个共享
// 的额外怪区）、szone 8 槽（5 魔法陷阱 + 场地 + 两侧灵摆），与引擎
// MR2020 区域编号一致。
export const ZONE_COORDS: Record<number, Record<string, any>> = {
  // Player 0（z 正方向，靠近摄像机；对应原版画面下半边）
  0: {
    mzone: [
      { x: -4.5, y: 0.02, z: 2.8 },
      { x: -2.3, y: 0.02, z: 2.8 },
      { x: -0.1, y: 0.02, z: 2.8 },
      { x: 2.1,  y: 0.02, z: 2.8 },
      { x: 4.3,  y: 0.02, z: 2.8 },
      // seq 5/6 = 共享的两格额外怪区（原版 vFieldMzone[0][5]/[6]）
      { x: -2.3, y: 0.02, z: 0.0 },
      { x: 2.1,  y: 0.02, z: 0.0 }
    ],
    szone: [
      { x: -4.5, y: 0.02, z: 5.2 },
      { x: -2.3, y: 0.02, z: 5.2 },
      { x: -0.1, y: 0.02, z: 5.2 },
      { x: 2.1,  y: 0.02, z: 5.2 },
      { x: 4.3,  y: 0.02, z: 5.2 },
      // seq 5 = 场地魔法区；seq 6/7 = 两侧灵摆区（vFieldSzone[0][5..7]）
      { x: -6.8, y: 0.02, z: 1.4 },
      { x: -6.8, y: 0.02, z: 4.0 },
      { x: 6.6,  y: 0.02, z: 4.0 }
    ],
    field:     { x: -6.8, y: 0.02, z: 1.4 },
    grave:     { x: 6.6,  y: 0.02, z: 1.4 },
    banish:    { x: 8.6,  y: 0.02, z: 1.4 },
    deck:      { x: 6.6,  y: 0.02, z: 6.6 },
    extra:     { x: -6.8, y: 0.02, z: 6.6 },
    emz_left:  { x: -2.3, y: 0.02, z: 0.0 },
    emz_right: { x: 2.1,  y: 0.02, z: 0.0 }
  },
  // Player 1（关于原点中心对称，即原版画面上半边）。额外怪区两格由双方
  // 共享，seq 5/6 的左右顺序随玩家视角镜像。
  1: {
    mzone: [
      { x: 4.3,  y: 0.02, z: -2.8 },
      { x: 2.1,  y: 0.02, z: -2.8 },
      { x: -0.1, y: 0.02, z: -2.8 },
      { x: -2.3, y: 0.02, z: -2.8 },
      { x: -4.5, y: 0.02, z: -2.8 },
      { x: 2.1,  y: 0.02, z: 0.0 },
      { x: -2.3, y: 0.02, z: 0.0 }
    ],
    szone: [
      { x: 4.3,  y: 0.02, z: -5.2 },
      { x: 2.1,  y: 0.02, z: -5.2 },
      { x: -0.1, y: 0.02, z: -5.2 },
      { x: -2.3, y: 0.02, z: -5.2 },
      { x: -4.5, y: 0.02, z: -5.2 },
      // seq 5 = 场地魔法区；seq 6/7 = 两侧灵摆区
      // （vFieldSzone[1][5..7] 原值；原版左右两半有 0.1 的固有不对称）
      { x: 6.6,  y: 0.02, z: -1.4 },
      { x: 6.6,  y: 0.02, z: -4.0 },
      { x: -6.8, y: 0.02, z: -4.0 }
    ],
    field:     { x: 6.6,  y: 0.02, z: -1.4 },
    grave:     { x: -6.8, y: 0.02, z: -1.4 },
    banish:    { x: -8.8, y: 0.02, z: -1.4 },
    deck:      { x: -6.8, y: 0.02, z: -6.6 },
    extra:     { x: 6.6,  y: 0.02, z: -6.6 },
    emz_left:  { x: -2.3, y: 0.02, z: 0.0 },
    emz_right: { x: 2.1,  y: 0.02, z: 0.0 }
  }
};