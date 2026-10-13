// DataManager::deck_sort_{lv,atk,def,name}, source/ygopro/gframe/data_manager.cpp.
export type DeckSortMode = 'level' | 'attack' | 'defense' | 'name';
const monsterType = (type: number) => type & 0x48020c0 ? type & 0x48020c1 : type & 0x31;
export function compareDeckCards(a: any, b: any, mode: DeckSortMode = 'level'): number {
  const code = Number(a.code) - Number(b.code);
  if (mode === 'name') return String(a.name) < String(b.name) ? -1 : String(a.name) > String(b.name) ? 1 : code;
  const kind = (a.type & 7) - (b.type & 7);
  if (kind) return kind;
  if ((a.type & 7) !== 1) return (a.type & 0xfffffff8) - (b.type & 0xfffffff8) || code;
  const type = monsterType(a.type) - monsterType(b.type);
  const level = (b.level ?? 0) - (a.level ?? 0);
  const attack = (b.attack ?? 0) - (a.attack ?? 0);
  const defense = (b.defense ?? 0) - (a.defense ?? 0);
  if (mode === 'attack') return attack || defense || level || type || code;
  if (mode === 'defense') return defense || attack || level || type || code;
  return type || level || attack || defense || code;
}
