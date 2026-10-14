// CheckSelectTribute/CheckSelectSum/get_sum_params, client_field.cpp.
export type FieldSelectionKind='tribute'|'sum'|'counter';
export interface FieldSelection {kind:FieldSelectionKind;data:any;cards:any[];selected:number[];counts:number[];}
export function sumValues(raw:number):number[] {
  const value=raw>>>0,high=value>>>16,low=value&0xffff;
  if(high&0x8000)return [value&0x7fffffff];
  return high>0&&high!==low?[low,high]:[low];
}
function values(s:FieldSelection,c:any):number[]{return s.kind==='tribute'?[...new Set([1,Math.max(1,Number(c.p)||1)])]:sumValues(Number(c.param)||0);}
export function fieldSelectionReady(s:FieldSelection,selected=s.selected):boolean {
  const {kind,data,cards}=s;
  if(kind==='counter')return s.counts.reduce((sum,n)=>sum+n,0)===data.count;
  if(kind==='sum'&&data.sumMode!==0){
    const all=[...(data.must||[]),...selected.map(i=>cards[i])];
    const low=all.reduce((sum,c)=>sum+Math.min(...values(s,c)),0),high=all.reduce((sum,c)=>sum+Math.max(...values(s,c)),0);
    return low>=data.acc||(all.length>0&&high>=data.acc&&high-Math.min(...all.map(c=>Math.max(...values(s,c))))<data.acc);
  }
  if(kind==='sum'&&(selected.length<data.min||selected.length>data.max))return false;
  const target=kind==='tribute'?data.max:data.acc;
  let sums=new Set([0]);
  for(const c of [...(kind==='sum'?data.must||[]:[]),...selected.map(i=>cards[i])]){
    const next=new Set<number>();for(const sum of sums)for(const n of values(s,c))if(sum+n<=target)next.add(sum+n);sums=next;
  }
  return kind==='tribute'?[...sums].some(sum=>sum>=data.min&&sum<=data.max):sums.has(data.acc);
}
export function canPickFieldCard(s:FieldSelection,index:number):boolean {
  if(!s.cards[index])return false;
  if(s.kind==='counter')return s.counts[index]<s.cards[index].cnt&&!fieldSelectionReady(s);
  if(s.selected.includes(index))return true;
  const chosen=[...s.selected,index];
  if(s.kind==='sum'&&s.data.sumMode!==0){
    // check_min: an added card must participate in a non-redundant completion.
    const fixed=[...(s.data.must||[]),...s.selected.map(i=>s.cards[i])];
    const lows=fixed.map(c=>Math.min(...values(s,c))),sum=lows.reduce((a,b)=>a+b,0);
    if(sum>=s.data.acc)return false;
    for(const contribution of values(s,s.cards[index])){
      const total=sum+contribution,smallest=Math.min(contribution,...lows);
      if(total>=s.data.acc){if(total-smallest<s.data.acc)return true;continue;}
      const min=s.data.acc-total,max=min+smallest-1;
      let sums=new Set([0]);
      for(let i=0;i<s.cards.length;i++){
        if(chosen.includes(i))continue;
        const n=Math.min(...values(s,s.cards[i])),next=new Set(sums);
        for(const value of sums)if(value+n<=max)next.add(value+n);sums=next;
      }
      if([...sums].some(value=>value>=min&&value<=max))return true;
    }
    return false;
  }
  if(fieldSelectionReady(s,chosen))return true;
  const {data,kind,cards}=s,max=kind==='tribute'?data.max:data.acc;
  let states=new Set(['0:0']);
  const required=[...(kind==='sum'?data.must||[]:[]),...chosen.map(i=>cards[i])];
  for(let i=0;i<required.length;i++){
    const next=new Set<string>(),isMust=kind==='sum'&&i<(data.must||[]).length;
    for(const state of states){const [sum,count]=state.split(':').map(Number);for(const n of values(s,required[i]))if(sum+n<=max)next.add(`${sum+n}:${count+(isMust?0:1)}`);}states=next;
  }
  for(let i=0;i<cards.length;i++){
    if(chosen.includes(i))continue;const next=new Set(states);
    for(const state of states){const [sum,count]=state.split(':').map(Number);if(count>=data.max)continue;for(const n of values(s,cards[i]))if(sum+n<=max)next.add(`${sum+n}:${count+1}`);}states=next;
  }
  return [...states].some(state=>{const [sum,count]=state.split(':').map(Number);return kind==='tribute'?sum>=data.min:sum===data.acc&&count>=data.min&&count<=data.max;});
}
