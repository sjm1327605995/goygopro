/** Screen-space hand composition; gameplay still addresses cards by seat/sequence. */
export function arenaHandLayout(sequence:number,count:number,width:number,height:number,near:boolean,hover=0,shuffle=0) {
  const cardHeight=near?Math.min(160,height*.205):Math.min(70,height*.083);
  const cardWidth=cardHeight*.7;
  const spread=Math.min(cardWidth*(near ? .74 : .59),Math.max(0,(width*(near ? .76 : .48)-cardWidth)/Math.max(1,count-1)));
  const offset=sequence-(count-1)/2;
  const normalized=count>1?offset/((count-1)/2):0;
  const arc=(1-normalized*normalized)*(near?11:4);
  return {
    x:width/2+offset*spread*(1-shuffle),
    y:near?height-cardHeight*.6-12-arc-cardHeight*.10*hover:cardHeight*.6+12+arc,
    height:cardHeight*(1+.24*hover),
    angle:(near?-1:1)*normalized*(near ? .13 : .075)*(1-hover),
  };
}
