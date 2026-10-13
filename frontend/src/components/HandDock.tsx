import React, { useSyncExternalStore } from 'react';
import {duelStore} from '../duel/store.ts';
import {eventBus} from '../wails_bridge.ts';
// The visible hand is rendered in the same 3D scene as the field. This list
// supplies keyboard/screen-reader access using each card's sequence, not code.
export default function HandDock({interactive=true}:{interactive?:boolean}) {
  const hand=useSyncExternalStore(duelStore.subscribe,duelStore.getState).hand;
  return <div className="hand-dock-container original-hand-accessibility"><div id="hand-cards-dock">
    {hand.map((card,seq)=>card?<button className="hand-card-item" data-code={card.code} data-sequence={seq} key={seq}
      disabled={!interactive||!card.code} onClick={(e)=>{const r=e.currentTarget.getBoundingClientRect();eventBus.emit('ui:hand_click',{code:card.code,seq,x:r.x,y:r.y});}}
    >卡片 {card.code}</button>:null)}
  </div></div>;
}
