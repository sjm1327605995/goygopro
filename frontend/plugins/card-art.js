import {readFile, stat} from 'node:fs/promises';
import {resolve, join} from 'node:path';

const MAX_BYTES = 8 * 1024 * 1024;
const CACHE_BYTES = 32 * 1024 * 1024;
const ROUTE = '/__goygopro/card-art/';
function imageType(bytes) {
  if(bytes[0]===0xff&&bytes[1]===0xd8&&bytes[2]===0xff)return 'image/jpeg';
  if(bytes.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))return 'image/png';
  if(bytes.toString('ascii',0,4)==='RIFF'&&bytes.toString('ascii',8,12)==='WEBP')return 'image/webp';
  return null;
}

// Development/preview-only endpoint. The caller can supply a card ID, never a
// path or upstream URL; Wails production continues to use Go's GetCardImage.
export function createCardArtHandler({resourceRoot, fetcher=globalThis.fetch}) {
  const roots=[process.env.YGOPRO_PICS_DIR,resourceRoot].filter(Boolean).map(p=>resolve(p));
  const folders=roots.flatMap(root=>['pics','expansions/pics','YGOPro/pics'].map(p=>join(root,p)));
  const cache=new Map(),inflight=new Map();
  let cacheBytes=0,active=0;
  const queue=[];
  async function remoteSlot(task) {
    if(active>=4){
      if(queue.length>=64)return null;
      await new Promise(resolve=>queue.push(resolve));
    } else active++;
    try{return await task();}finally{const next=queue.shift();if(next)next();else active--;}
  }
  async function load(code) {
    for(const folder of folders)for(const suffix of ['', '-1', '-2'])for(const extension of ['jpg','png','webp']){
      const path=join(folder,`${code}${suffix}.${extension}`);
      try{
        const info=await stat(path);
        if(!info.isFile()||!info.size||info.size>MAX_BYTES)continue;
        const bytes=await readFile(path),type=imageType(bytes);
        if(type&&bytes.length<=MAX_BYTES)return {bytes,type};
      }catch{/* Missing art is expected; try the next approved location. */}
    }
    if(cache.has(code))return cache.get(code);
    const result=await remoteSlot(async()=>{
      for(const directory of ['cards','cards_small']){
        const controller=new AbortController();
        const timer=setTimeout(()=>controller.abort(),7000);
        try{
          const response=await fetcher(`https://images.ygoprodeck.com/images/${directory}/${code}.jpg`,{
            signal:controller.signal,redirect:'error',headers:{'User-Agent':'goygopro-preview/1.0'},
          });
          if(!response.ok||!response.headers.get('content-type')?.startsWith('image/')||!response.body){await response.body?.cancel();continue;}
          const reader=response.body.getReader(),chunks=[];let size=0;
          try{
            for(;;){const {done,value}=await reader.read();if(done)break;size+=value.length;if(size>MAX_BYTES){await reader.cancel();break;}chunks.push(value);}
          }finally{reader.releaseLock();}
          if(size>MAX_BYTES)continue;
          const bytes=Buffer.concat(chunks),type=imageType(bytes);
          if(type&&bytes.length)return {bytes,type};
        }catch{/* Offline, rejected response or timeout: use the fallback image size. */}
        finally{clearTimeout(timer);}
      }
      return null;
    });
    if(result){
      while(cacheBytes+result.bytes.length>CACHE_BYTES&&cache.size){const first=cache.keys().next().value;cacheBytes-=cache.get(first).bytes.length;cache.delete(first);}
      cache.set(code,result);cacheBytes+=result.bytes.length;
    }
    return result;
  }
  return async (request,response,next)=>{
    const pathname=new URL(request.url,'http://localhost').pathname;
    if(!pathname.startsWith(ROUTE)){next();return;}
    if(!['GET','HEAD'].includes(request.method)){response.writeHead(405,{'Allow':'GET, HEAD'});response.end();return;}
    const id=pathname.slice(ROUTE.length);
    if(!/^[1-9]\d{0,9}$/.test(id)||Number(id)>0xffffffff){response.writeHead(400);response.end('Invalid card ID');return;}
    try{
      if(!inflight.has(id)){
        const promise=load(id).finally(()=>inflight.delete(id));inflight.set(id,promise);
      }
      const result=await inflight.get(id);
      if(response.destroyed)return;
      if(!result){response.writeHead(404,{'Cache-Control':'no-store'});response.end();return;}
      response.writeHead(200,{'Content-Type':result.type,'Content-Length':result.bytes.length,'Cache-Control':'no-store','X-Content-Type-Options':'nosniff'});
      response.end(request.method==='HEAD'?undefined:result.bytes);
    }catch{
      if(!response.headersSent)response.writeHead(502,{'Cache-Control':'no-store'});
      response.end();
    }
  };
}

export default function cardArtPlugin(frontendRoot) {
  const handler=createCardArtHandler({resourceRoot:resolve(frontendRoot,'..')});
  return {name:'goygopro-preview-card-art',configureServer(server){server.middlewares.use(handler);},configurePreviewServer(server){server.middlewares.use(handler);}};
}
