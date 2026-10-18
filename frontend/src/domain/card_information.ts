import {TYPE_MONSTER,TYPE_LINK,TYPE_XYZ,TYPE_PENDULUM,formatRace,formatAttribute,formatLinkMarker} from './constants.ts';

// strings.conf !system 1050..1076, in bit order (DataManager::FormatType).
const TYPES=['怪兽','魔法','陷阱','？？？','通常','效果','融合','仪式','陷阱怪兽','灵魂','同盟','二重','调整','同调','衍生物','？？？','速攻','永续','装备','场地','反击','反转','卡通','超量','灵摆','特殊召唤','连接'];
export function formatCardType(type:number):string {
  return TYPES.filter((_,bit)=>(type&(2**bit))!==0).join('|')||'？？？';
}
export function formatCardName(info:any):string {
  return `${info.originalName??info.name??'？？？'}[${String(info.alias||info.code).padStart(8,'0')}]`;
}
export function formatCardTypeLine(info:any):string {
  const kind=`[${formatCardType(info.type||0)}]`;
  return (info.type&TYPE_MONSTER)?`${kind} ${formatRace(info.race||0)}/${formatAttribute(info.attribute||0)}`:kind;
}
export function formatCardStats(info:any):string {
  const stat=(value:any)=>typeof value==='number'&&value>=0?String(value):'?';
  const link=(info.type&TYPE_LINK)!==0;
  const form=link?'LINK-':(info.type&TYPE_XYZ)?'☆':'★';
  const ad=link?`${stat(info.attack)}/-   ${formatLinkMarker(info.linkMarker||0)}`:`${stat(info.attack)}/${stat(info.defense)}`;
  return `[${form}${info.level??0}] ${ad}${(info.type&TYPE_PENDULUM)?`   ${info.lscale||0}/${info.rscale||0}`:''}`;
}

export function splitPendulumDescription(input:string):[string,string] {
  const text=String(input??'').replace(/←.*?→/g,'').trim();
  const marker=/【(?:怪兽效果|怪獸效果|モンスター効果)】|\[\s*(?:Monster Effect|Efecto de Monstruo)\s*\]/i;
  const separator=marker.exec(text)??/\r?\n[-─━]{3,}\r?\n/.exec(text);
  if(!separator)return ['',text];
  const pendulum=text.slice(0,separator.index).replace(/【(?:灵摆效果|靈擺效果|Ｐ効果|P効果)】|\[\s*(?:Pendulum Effect|Efecto de Péndulo)\s*\]/gi,'').trim();
  const monster=text.slice(separator.index+separator[0].length).trim();
  return [pendulum,monster];
}
