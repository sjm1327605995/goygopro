import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,readFile,unlink,rmdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {createCardArtHandler} from '../frontend/plugins/card-art.js';

test('preview card art: local-first, coalesced upstream, missing retry and constrained route',async()=>{
  const root=await mkdtemp(join(tmpdir(),'goygopro-card-art-'));
  const pics=join(root,'pics');await mkdir(pics);
  const image=await readFile(new URL('../frontend/smoke/assets/blue-eyes.jpg',import.meta.url));
  const file=join(pics,'89631139.jpg');await writeFile(file,image);
  const calls=[];let missing=true;
  const handler=createCardArtHandler({resourceRoot:root,fetcher:async url=>{
    calls.push(url);await new Promise(resolve=>setTimeout(resolve,15));
    if(url.includes('/1002.jpg')&&missing)return new Response('missing',{status:404});
    if(url.includes('/1003.jpg'))return new Response('<html>error</html>',{headers:{'content-type':'image/jpeg'}});
    return new Response(image,{headers:{'content-type':'image/jpeg'}});
  }});
  const server=createServer((request,response)=>void handler(request,response,()=>{response.writeHead(404);response.end();}));
  server.listen(0,'127.0.0.1');await once(server,'listening');const base=`http://127.0.0.1:${server.address().port}`;
  try{
    const local=await fetch(`${base}/__goygopro/card-art/89631139`);
    assert.equal(local.status,200);assert.equal(local.headers.get('content-type'),'image/jpeg');
    assert.deepEqual(Buffer.from(await local.arrayBuffer()),image);assert.equal(calls.length,0);
    const requests=await Promise.all(Array.from({length:8},()=>fetch(`${base}/__goygopro/card-art/1001`)));
    assert.ok(requests.every(r=>r.status===200));assert.equal(calls.length,1);
    assert.ok(calls.every(url=>url.startsWith('https://images.ygoprodeck.com/images/')));
    assert.equal((await fetch(`${base}/__goygopro/card-art/1002`)).status,404);
    missing=false;assert.equal((await fetch(`${base}/__goygopro/card-art/1002`)).status,200);
    assert.equal((await fetch(`${base}/__goygopro/card-art/1003`)).status,404);
    for(const id of ['0','4294967296','1001.jpg','https://example.com','1001%2fsecret'])assert.equal((await fetch(`${base}/__goygopro/card-art/${id}`)).status,400);
    const head=await fetch(`${base}/__goygopro/card-art/89631139`,{method:'HEAD'});assert.equal(head.status,200);assert.equal((await head.arrayBuffer()).byteLength,0);
    assert.equal((await fetch(`${base}/__goygopro/card-art/1001`,{method:'POST'})).status,405);
  }finally{server.closeAllConnections();await new Promise(resolve=>server.close(resolve));await unlink(file);await rmdir(pics);await rmdir(root);}
});
