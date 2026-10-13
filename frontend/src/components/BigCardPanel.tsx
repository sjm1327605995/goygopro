import React, {useEffect,useState} from 'react';
import {WailsBridge} from '../wails_bridge.ts';
import UiButton from '../ui/UiButton.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';

// DeckBuilder::ShowBigCard/ZoomBigCard: native picture dimensions, 0.2..4x,
// wheel zoom anchored at the pointer, button zoom anchored at 44.4% height.
export default function BigCardPanel({code,onClose}:{code:number;onClose:()=>void}) {
  const [pic,setPic]=useState('');
  const [view,setView]=useState({x:0,y:0,width:177,height:254,scale:1});
  useEffect(()=>{let alive=true;WailsBridge.getCardImage(code).then(p=>{if(alive&&p)setPic(p.url);});return()=>{alive=false;};},[code]);
  useEffect(()=>{const close=(e:MouseEvent)=>{e.preventDefault();e.stopPropagation();onClose();};document.addEventListener('contextmenu',close,true);return()=>document.removeEventListener('contextmenu',close,true);},[onClose]);
  const zoom=(delta:number,x?:number,y?:number,reset=false)=>setView(v=>{
    const scale=reset?1:Math.max(.2,Math.min(4,v.scale+delta));
    const cx=x??v.x+v.width*v.scale/2,cy=y??v.y+v.height*v.scale*.444;
    const rx=(cx-v.x)/(v.width*v.scale),ry=(cy-v.y)/(v.height*v.scale);
    return {...v,scale,x:cx-v.width*scale*rx,y:cy-v.height*scale*ry};
  });
  return <div id="deck-zoom-overlay" style={{position:'fixed',inset:0,zIndex:2000,pointerEvents:'none'}}>
    {pic&&<img id="deck-big-card" src={pic} alt={String(code)} draggable={false}
      onLoad={e=>{const i=e.currentTarget;setView({width:i.naturalWidth,height:i.naturalHeight,scale:1,x:(innerWidth-i.naturalWidth)/2,y:(innerHeight-i.naturalHeight)/2});}}
      onWheel={e=>{e.preventDefault();zoom(e.deltaY<0?.1:-.1,e.clientX,e.clientY);}}
      style={{position:'absolute',left:view.x,top:view.y,width:view.width*view.scale,height:view.height*view.scale,pointerEvents:'auto'}}/>}
    <DesignSpace>
      {[['原始尺寸',100,()=>zoom(0,undefined,undefined,true)],['放大',140,()=>zoom(.2)],['缩小',180,()=>zoom(-.2)],['关闭',230,onClose]].map(([label,top,action])=><UiButton key={String(label)} id={label==='关闭'?'deck-big-card-close':undefined} style={{position:'absolute',left:205,top:Number(top),width:90,height:35,pointerEvents:'auto'}} onClick={action as ()=>void}>{String(label)}</UiButton>)}
    </DesignSpace>
  </div>;
}
