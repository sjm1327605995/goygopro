import { ZONE_COORDS } from './field3d_geometry.ts';

// materials.cpp -> Three world: (x,y,z) -> (2(x-4),2z,2y).
export function fieldZonePosition(player: number, loc: string, seq: number, duelRule: number): {x:number;y:number;z:number} | null {
  const modern = duelRule >= 4;
  const point = (x: number, y: number) => ({x:2*(x-4), y:0.02, z:2*y});
  if (loc === 'szone' && seq >= 5) {
    const positions = player === 0
      ? modern ? [[0.6,2],[0.6,0.7],[8.3,0.7]] : [[0.6,0.7],[0.6,2],[7.3,2]]
      : modern ? [[7.3,-2],[7.3,-0.7],[-0.4,-0.7]] : [[7.3,-0.7],[7.3,-2],[0.6,-2]];
    const p = positions[seq-5];
    return p ? point(p[0],p[1]) : null;
  }
  if (loc === 'grave') return player === 0 ? point(7.3,modern?2:0.7) : point(0.6,modern?-2:-0.7);
  if (loc === 'banish') return player === 0 ? point(modern?7.3:8.3,0.7) : point(modern?0.6:-0.4,-0.7);
  if (loc === 'field') return fieldZonePosition(player,'szone',5,duelRule);
  const zone = ZONE_COORDS[player]?.[loc];
  if (!zone || (!modern && loc === 'mzone' && seq >= 5)) return null;
  return Array.isArray(zone) ? zone[seq] || null : zone;
}

// ClientField::GetCardLocation LOCATION_HAND. Rotation is viewer-dependent and
// applied by DuelField3D after this position lookup.
export function handPose(player: number, sequence: number, count: number, hovered = false) {
  const qx = player === 0
    ? count <= 6 ? (5.5-0.8*count)/2 + 1.55 + sequence*0.8 : 1.9 + sequence*4/(count-1)
    : count <= 6 ? 6.25-(5.5-0.8*count)/2-sequence*0.8 : 5.9-sequence*4/(count-1);
  return {
    x:2*(qx-4), y:2*((hovered?0.656:0.5)+(player===0?1:-1)*0.001*sequence),
    z:2*(player===0?(hovered?3.84:4):(hovered?-3.56:-3.4)),
  };
}
