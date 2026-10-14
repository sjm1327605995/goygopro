import React from 'react';
import {AVAIL_OCG,AVAIL_TCG,AVAIL_SC,AVAIL_CUSTOM,AVAIL_OCGTCG} from '../domain/constants.ts';
// DrawThumb's tOT atlas: unavailable OCG/TCG (rows 0/1), available exclusives (2/3).
export default function CardAvailabilityBadge({ot,filter,defaultOT}:{ot:number;filter:number;defaultOT:number}) {
  const masks:Record<number,number>={4:AVAIL_OCG,5:AVAIL_TCG,6:AVAIL_SC,7:AVAIL_CUSTOM,8:AVAIL_OCGTCG};
  const active=filter>=4;
  const available=!active||(filter===8?(ot&3)===3:(ot&(masks[filter]||0))!==0);
  let row=-1;
  if(active&&available){if((ot&3)===1)row=2;else if((ot&3)===2)row=3;}
  else if(!available||!(ot&defaultOT)){if(ot&1)row=0;else if(ot&2)row=1;}
  if(row<0&&available)return null;
  const banned=row<0;
  return <span className="deck-availability-badge" aria-label={banned?'不可用':row>=2?'卡片发行范围':'超出卡片允许范围'}
    style={{position:'absolute',left:7,top:50,width:30,height:15,backgroundImage:`url('textures/${banned?'lim':'ot'}.png')`,backgroundSize:banned?'60px 30px':'30px 60px',backgroundPosition:banned?'0 0':`0 ${-15*row}px`,pointerEvents:'none'}}/>;
}
