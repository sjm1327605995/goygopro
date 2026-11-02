import {useEffect,useState} from 'react';
import {WailsBridge} from '../wails_bridge.ts';

export function useCardImage(code:number,revision=0) {
  const [pic,setPic]=useState<string|null>(null);
  const [loading,setLoading]=useState(true);
  useEffect(()=>{
    let alive=true;setPic(null);setLoading(true);
    WailsBridge.getCardImage(code).then(result=>{if(alive)setPic(result?.url||null);})
      .catch(()=>{if(alive)setPic(null);}).finally(()=>{if(alive)setLoading(false);});
    return()=>{alive=false;};
  },[code,revision]);
  return {pic,loading,onError:()=>{setPic(null);setLoading(false);}};
}
