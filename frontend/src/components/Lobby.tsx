import React, { useEffect, useRef, useState } from 'react';
import { WailsBridge, eventBus } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';

interface LobbyProps {
  onNavigate: (screen: string) => void;
  onDuelStart?: () => void;
}

interface PlayerSeat {
  name: string;
  status: string;
  color: string;
  ready: boolean;
}

const EMPTY_SEAT = (): PlayerSeat => ({ name: '', status: '等待玩家加入...', color: '#9ca3af', ready: false });

// 建房参数取值与 vendored ygopro 对齐：
//   duelRule: ocgcore/common.go MASTER_RULE3=3 / NEW_MASTER_RULE=4 /
//             MASTER_RULE_2020=5（服务端按 DuelFlag<<16 透传给 ocgcore）
//   mode:     network.h MODE_SINGLE=0 / MODE_MATCH=1 / MODE_TAG=2
const DUEL_RULES: { value: number; label: string }[] = [
  { value: 5, label: '大师规则 2020 (MR5)' },
  { value: 4, label: '新大师规则 (NMR)' },
  { value: 3, label: '大师规则 3' },
];

const DUEL_MODES: { value: number; label: string }[] = [
  { value: 0, label: '单局赛（1 局）' },
  { value: 1, label: '比赛赛（三局两胜）' },
  { value: 2, label: '组队赛（2 对 2）' },
];

// 卡片允许（SysString 1481+rule）：OCG/TCG/简体中文/自定义卡片
const CARD_RULES: { value: number; label: string }[] = [
  { value: 0, label: 'ＯＣＧ' },
  { value: 1, label: 'ＴＣＧ' },
  { value: 2, label: '简体中文' },
  { value: 3, label: '自定义卡片' },
];

// 规则信息面板（stHostPrepRule，duelclient.cpp:453-490）文案
const lfNameOf = (hash: number, lists: { hash: number; name: string }[]): string =>
  (lists.find((l) => l.hash === hash) || { name: 'N/A' }).name;
const roomRuleLines = (info: any, lists: { hash: number; name: string }[]): string[] => {
  if (!info) return [];
  const lines: string[] = [];
  lines.push(`禁限卡表：${lfNameOf(Number(info.LFList ?? info.lflist ?? 0), lists)}`);
  lines.push(`卡片允许：${(CARD_RULES[Number(info.Rule ?? info.rule ?? 0)] || CARD_RULES[0]).label}`);
  lines.push(`决斗模式：${(DUEL_MODES[Number(info.Mode ?? info.mode ?? 0)] || DUEL_MODES[0]).label}`);
  const tl = Number(info.TimeLimit ?? info.timeLimit ?? 0);
  if (tl) lines.push(`每回合时间：${tl}`);
  lines.push('==========');
  lines.push(`初始基本分：${Number(info.StartLp ?? info.startLp ?? 0)}`);
  lines.push(`初始手卡数：${Number(info.StartHand ?? info.startHand ?? 0)}`);
  lines.push(`每回合抽卡：${Number(info.DrawCount ?? info.drawCount ?? 0)}`);
  // DEFAULT_DUEL_RULE = CURRENT_RULE = 5（ocgcore/common.h），非默认才显示
  const dr = Number(info.DuelRule ?? info.duelRule ?? 0);
  const ruleName = (DUEL_RULES.find((r) => r.value === dr) || { label: '' }).label;
  if (dr && dr !== 5 && ruleName) lines.push(`*${ruleName}`);
  if (Number(info.NoCheckDeck ?? info.noCheckDeck ?? 0)) lines.push('*不检查卡组');
  if (Number(info.NoShuffleDeck ?? info.noShuffleDeck ?? 0)) lines.push('*不洗切卡组');
  return lines;
};

// ---- STOC_ERROR_MSG 文案（原版 duelclient.cpp:261-368 的 sysString 1400 段）----
// 仓库没有 strings.conf，按原版键位硬编码中文。
const ERRMSG_JOINERROR = 0x1;
const ERRMSG_DECKERROR = 0x2;
const ERRMSG_SIDEERROR = 0x3;
const ERRMSG_VERERROR = 0x4;
const DECKERROR_LFLIST = 0x1;
const DECKERROR_OCGONLY = 0x2;
const DECKERROR_TCGONLY = 0x3;
const DECKERROR_UNKNOWNCARD = 0x4;
const DECKERROR_CARDCOUNT = 0x5;
const DECKERROR_MAINCOUNT = 0x6;
const DECKERROR_EXTRACOUNT = 0x7;
const DECKERROR_SIDECOUNT = 0x8;
const DECKERROR_NOTAVAIL = 0x9;
const MAX_CARD_ID = 0x0fffffff;

const describeErrorMsg = (msg: number, code: number, cardName: string | null): string => {
  switch (msg) {
    case ERRMSG_JOINERROR:
      if (code === 0) return '房间人数已满，无法加入。';
      if (code === 1) return '房间密码错误。';
      return '无法加入该房间。';
    case ERRMSG_DECKERROR: {
      const flag = code >>> 28;
      const cardCode = code & MAX_CARD_ID;
      const name = cardName || `未知卡片 #${cardCode}`;
      switch (flag) {
        case DECKERROR_LFLIST: return `卡片【${name}】在禁限卡表中受限，无法使用。`;
        case DECKERROR_OCGONLY: return `卡片【${name}】仅能在 OCG 环境使用。`;
        case DECKERROR_TCGONLY: return `卡片【${name}】仅能在 TCG 环境使用。`;
        case DECKERROR_UNKNOWNCARD: return `未知的卡片【${name}】(${cardCode})。`;
        case DECKERROR_CARDCOUNT: return `卡片【${name}】的数量超过限制。`;
        case DECKERROR_MAINCOUNT: return `主卡组数量必须在 ${cardCode} 张以上。`;
        case DECKERROR_EXTRACOUNT:
          return cardCode > 0
            ? `额外卡组数量必须在 ${cardCode} 张以上。`
            : '额外卡组数量必须为 0。';
        case DECKERROR_SIDECOUNT: return `副卡组数量必须在 ${cardCode} 张以上。`;
        case DECKERROR_NOTAVAIL: return `卡片【${name}】不能在这个卡组中使用。`;
        default: return '卡组不合法。';
      }
    }
    case ERRMSG_SIDEERROR:
      return '副卡组数量不符，无法开始下一局。';
    case ERRMSG_VERERROR:
      return `与服务器版本不符（${code >> 12}.${(code >> 4) & 0xff}.${code & 0xf}），无法连接。`;
    default:
      return `未知错误 (msg=${msg}, code=${code})。`;
  }
};

// 聊天身份标签（原版 drawing.cpp / netserver.cpp:380-385：player<4 决斗者、
// 8=系统、10-19 观战者）
const chatIdentity = (player: number, seats: PlayerSeat[]): { name: string; color: string } => {
  if (player < 4) {
    const seat = seats[player];
    if (seat && seat.name) return { name: seat.name, color: player === 0 ? '#7dd3fc' : '#fca5a5' };
    return { name: `决斗者 ${player + 1}`, color: player === 0 ? '#7dd3fc' : '#fca5a5' };
  }
  if (player === 8) return { name: '系统', color: '#fbbf24' };
  if (player >= 10 && player <= 19) return { name: `观战者 ${player - 9}`, color: '#9ca3af' };
  return { name: `频道 ${player}`, color: '#9ca3af' };
};

export default function Lobby({ onNavigate, onDuelStart }: LobbyProps) {
  // 波 H：主机地址拆成 host/port 两个输入（原版 ebJoinHost/ebJoinPort，game.cpp:226-228）
  const [joinHost, setJoinHost] = useState('127.0.0.1');
  const [joinPort, setJoinPort] = useState('7911');
  // 窗口相位：LAN 窗（未连接）→ 建房窗（建立主机）→ 等候区窗（连接/建房/加入成功）
  const [createOpen, setCreateOpen] = useState(false);
  const [username, setUsername] = useState('YugiMuto');
  const [password, setPassword] = useState('');
  const [localPort, setLocalPort] = useState('7911');
  const [roomName, setRoomName] = useState('Championship Match');
  const [roomPass, setRoomPass] = useState('');
  const [joinPass, setJoinPass] = useState('');
  const [duelRule, setDuelRule] = useState(5);
  const [duelMode, setDuelMode] = useState(0);
  // 建房高级参数（gframe wCreateHost：cbLFlist/cbRule/ebStartLP/ebStartHand/
  // ebDrawCount/ebTimeLimit/chkNoCheckDeck/chkNoShuffleDeck）
  const [lfLists, setLfLists] = useState<{ hash: number; name: string }[]>([]);
  const [lflist, setLflist] = useState(0);
  const [cardRule, setCardRule] = useState(0);
  const [startLp, setStartLp] = useState('8000');
  const [startHand, setStartHand] = useState('5');
  const [drawCount, setDrawCount] = useState('1');
  const [timeLimit, setTimeLimit] = useState('180');
  const [noCheckDeck, setNoCheckDeck] = useState(false);
  const [noShuffleDeck, setNoShuffleDeck] = useState(false);
  // 规则信息面板（stHostPrepRule）：来自 stoc:join_game 的 HostInfo
  const [roomRuleInfo, setRoomRuleInfo] = useState<any>(null);

  const [connected, setConnected] = useState(false);
  const [inRoom, setInRoom] = useState(false);
  const [isHost, setIsHost] = useState(false);
  const [selfType, setSelfType] = useState(0);
  const [isReady, setIsReady] = useState(false);
  const [decks, setDecks] = useState<string[]>([]);
  const [pickedDeck, setPickedDeck] = useState('');
  const [seats, setSeats] = useState<PlayerSeat[]>([EMPTY_SEAT(), EMPTY_SEAT()]);
  const [watchCount, setWatchCount] = useState(0);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [messages, setMessages] = useState<string[]>([]);
  const [chat, setChat] = useState('');

  const chatBoxRef = useRef<HTMLDivElement | null>(null);
  // Observer（selftype>1，gframe 语义）：无准备/卡组操作，只能观战或转回决斗者
  const isObserver = selfType > 1;

  useEffect(() => {
    const onTypeChange = (data: any) => {
      // STOC_TYPE_CHANGE：低 4 位=selftype（0/1=决斗者，>1=观战者），bit4=宿主
      const self = Number(data.pos ?? 0);
      setSelfType(self);
      setIsHost(!!data.isHost);
      if (self > 1) {
        setIsReady(false);
      }
    };
    const onPlayerEnter = (data: any) => {
      const pos = Number(data.pos);
      if (pos > 1) return;
      setSeats((prev) => {
        const next = [...prev];
        next[pos] = { name: data.name || '', status: '已连接', color: '#10b981', ready: false };
        return next;
      });
    };
    // STOC_HS_PLAYER_CHANGE（duelclient.cpp:869-932）：status 低 4 位是事件、
    // 高 4 位是座位。Go 侧已拆出 pos/status。
    const onPlayerChange = (data: any) => {
      const pos = Number(data.pos);
      const state = Number(data.status);
      setSeats((prev) => {
        const next = [...prev];
        if (state < 8) {
          // 座位平移：pos 座位的玩家移到 state 座位
          const moving = prev[pos];
          if (pos < 2) next[pos] = EMPTY_SEAT();
          if (state < 2) next[state] = { ...moving, ready: false };
        } else if (state === 0x9) { // PLAYERCHANGE_READY
          if (pos < 2) next[pos] = { ...prev[pos], ready: true, status: '已准备', color: '#10b981' };
        } else if (state === 0xa) { // PLAYERCHANGE_NOTREADY
          if (pos < 2) next[pos] = { ...prev[pos], ready: false, status: '未准备', color: '#9ca3af' };
        } else if (state === 0xb) { // PLAYERCHANGE_LEAVE：清座位
          if (pos < 2) next[pos] = EMPTY_SEAT();
        } else if (state === 0x8) { // PLAYERCHANGE_OBSERVE：转观战，清座位
          if (pos < 2) next[pos] = EMPTY_SEAT();
        }
        return next;
      });
      if (pos === selfType) {
        if (state === 0x9) setIsReady(true);
        else if (state === 0xa || state === 0xb) setIsReady(false);
      }
    };
    const onWatchChange = (data: any) => {
      setWatchCount(Number(data.count) || 0);
    };
    const onErrorMsg = async (data: any) => {
      const msg = Number(data.msg);
      const code = Number(data.code);
      if (msg === ERRMSG_DECKERROR) {
        const cardCode = code & MAX_CARD_ID;
        let name: string | null = null;
        try {
          const info = await WailsBridge.getCard(cardCode);
          if (info && info.name) name = info.name;
        } catch { /* 名字解析失败则显示卡号 */ }
        setErrMsg(describeErrorMsg(msg, code, name));
      } else {
        setErrMsg(describeErrorMsg(msg, code, null));
      }
      // 加入/版本错误：回到连接界面（原版还重新 enable 房间按钮）
      if (msg === ERRMSG_JOINERROR || msg === ERRMSG_VERERROR) {
        setInRoom(false);
        setIsHost(false);
      }
    };
    const onDuelStartEvent = () => { if (onDuelStart) onDuelStart(); };
    // STOC_JOIN_GAME：HostInfo 展示在规则信息面板（duelclient.cpp:453-490）。
    // Go emit 的是 STOCJoinGame 结构体（Go 字段名直传），兼容大小写两种键
    const onJoinGame = (data: any) => {
      const info = data.Info ?? data.info ?? data;
      setRoomRuleInfo(info && typeof info === 'object' ? info : null);
    };
    // 对局彻底结束回等候区：重置准备状态
    const onDuelEnd = () => {
      setIsReady(false);
      setSeats((prev) => prev.map((s) => ({ ...s, ready: false, status: s.name ? '已连接' : s.status })));
      setInRoom(true);
    };
    const onChat = (data: any) => {
      const id = chatIdentity(Number(data.player), seats);
      setMessages((m) => [...m, `[${id.name}]: ${data.msg}`]);
    };

    eventBus.on('stoc:type_change', onTypeChange);
    eventBus.on('stoc:player_enter', onPlayerEnter);
    eventBus.on('stoc:player_change', onPlayerChange);
    eventBus.on('stoc:watch_change', onWatchChange);
    eventBus.on('stoc:error_msg', onErrorMsg);
    eventBus.on('stoc:duel_start', onDuelStartEvent);
    eventBus.on('stoc:duel_end', onDuelEnd);
    eventBus.on('stoc:chat', onChat);
    eventBus.on('stoc:join_game', onJoinGame);

    // Without this cleanup every re-mount (and every App re-render, since
    // onDuelStart changes identity) stacks another set of listeners: chat
    // messages appear twice, duel_start navigates twice.
    return () => {
      eventBus.off('stoc:type_change', onTypeChange);
      eventBus.off('stoc:player_enter', onPlayerEnter);
      eventBus.off('stoc:player_change', onPlayerChange);
      eventBus.off('stoc:watch_change', onWatchChange);
      eventBus.off('stoc:error_msg', onErrorMsg);
      eventBus.off('stoc:duel_start', onDuelStartEvent);
      eventBus.off('stoc:duel_end', onDuelEnd);
      eventBus.off('stoc:chat', onChat);
      eventBus.off('stoc:join_game', onJoinGame);
    };
  }, [onDuelStart, selfType, seats]);

  useEffect(() => {
    if (chatBoxRef.current) chatBoxRef.current.scrollTop = chatBoxRef.current.scrollHeight;
  }, [messages]);

  // 禁限卡表下拉（gframe cbLFlist 数据源；仓库无 lflist.conf 时只有 N/A）
  useEffect(() => {
    let alive = true;
    WailsBridge.listLFLists().then((lists) => {
      if (alive && lists && lists.length > 0) {
        setLfLists(lists);
        setLflist(lists[0].hash);
      }
    });
    return () => { alive = false; };
  }, []);

  // 大厅记忆预填（原版 game.cpp 点击连接/建房时保存 nickname/lasthost/lastport/
  // gamename/serverport/lastdeck，启动时回填输入框）。lastcategory 无对应 UI，
  // 卡组分类体系落地（波 D）时再接。
  useEffect(() => {
    let alive = true;
    settingsStore.load().then(() => {
      if (!alive) return;
      const s = settingsStore.getSnapshot();
      // 原版 lasthost/lastport 分两个键存；我们的输入框是 "IP:端口" 合一
      const host = s.lasthost ? String(s.lasthost) : '';
      const port = s.lastport ? String(s.lastport) : '';
      if (host) setJoinHost(host);
      if (port) setJoinPort(port);
      if (s.nickname) setUsername(String(s.nickname));
      if (s.serverport) setLocalPort(String(s.serverport));
      if (s.gamename) setRoomName(String(s.gamename));
      if (s.lastdeck) setPickedDeck(String(s.lastdeck));
    });
    return () => { alive = false; };
  }, []);

  const connectServer = async (): Promise<void> => {
    const addr = `${joinHost.trim() || '127.0.0.1'}:${joinPort.trim() || '7911'}`;
    const res = await WailsBridge.connectServer(addr, username.trim() || 'Duelist', password.trim());
    if (res.success) {
      setConnected(true);
      setSeats((prev) => {
        const next = [...prev];
        next[0] = { ...next[0], name: username.trim() || 'Duelist' };
        return next;
      });
      // 大厅记忆落盘（原版在点击连接时保存这组键）
      settingsStore.set('lasthost', joinHost.trim() || '127.0.0.1');
      settingsStore.set('lastport', joinPort.trim() || '7911');
      settingsStore.set('nickname', username.trim() || 'Duelist');
    } else {
      setErrMsg(`连接失败：${res.error}`);
    }
  };

  const startLocalServer = async (): Promise<void> => {
    const port = parseInt(localPort, 10) || 7911;
    const res = await WailsBridge.startLocalServer(port);
    if (res.success) {
      setJoinHost('127.0.0.1');
      setJoinPort(String(port));
      setConnected(true);
      settingsStore.set('serverport', port);
    } else {
      setErrMsg(`启动本地服务器失败：${res.error}`);
    }
  };

  const createRoom = async (): Promise<void> => {
    // duelRule/mode 来自界面下拉（此前硬编码 duelRule:5，两个下拉纯装饰）。
    // 其余高级参数同样来自界面（gframe wCreateHost 的 ebStartLP 等控件）；
    // 非法/空值交给 Go 侧兜底回默认（app.go CreateGame 的 clamp）。
    const req = {
      lflist, rule: cardRule, mode: duelMode, duelRule,
      startLp: Number(startLp) || 8000,
      startHand: Number(startHand) || 5,
      drawCount: Number(drawCount) || 1,
      timeLimit: Number(timeLimit) || 180,
      noCheckDeck, noShuffleDeck,
    };
    const res = await WailsBridge.createGame(req, roomName.trim() || 'Duel Room', roomPass.trim());
    if (res.success) {
      setIsHost(true);
      setInRoom(true);
      setCreateOpen(false); // 建房成功 → 进等候区窗
      settingsStore.set('gamename', roomName.trim() || 'Duel Room');
    }
  };

  // 加入房间：CTOS_JOIN_GAME（gameID 由服务端房间列表决定，本地单房间用 0）
  const joinRoom = async (): Promise<void> => {
    const res = await WailsBridge.joinGame(joinPass.trim());
    if (res.success) setInRoom(true);
    // 失败时服务端回 STOC_ERROR_MSG（JOINERROR），由 error_msg 弹窗呈现
  };

  // 离开房间：CTOS_LEAVE_GAME，重置等候区状态
  const leaveRoom = (): void => {
    WailsBridge.leaveGame();
    setInRoom(false);
    setIsHost(false);
    setIsReady(false);
    setRoomRuleInfo(null);
    setSeats([EMPTY_SEAT(), EMPTY_SEAT()]);
    setSeats((prev) => {
      const next = [...prev];
      next[0] = { ...next[0], name: username.trim() || 'Duelist' };
      return next;
    });
  };

  // 观战/回决斗（CTOS_HS_TOOBSERVER / CTOS_HS_TODUELIST）
  const watchAsObserver = (): void => WailsBridge.toObserver();
  const backToDuelist = (): void => WailsBridge.toDuelist();

  // 宿主踢人（CTOS_HS_KICK + 座位号；服务端仅准备阶段受理）
  const kickSeat = (pos: number): void => WailsBridge.kickPlayer(pos);

  // CTOS_UPDATE_DECK 握手：准备前必须上报卡组（联机必需，服务端
  // single_duel.go 用它填充 pDeck 并标 ready）。
  const toggleReady = async (): Promise<void> => {
    const next = !isReady;
    if (next) {
      if (!pickedDeck) { setErrMsg('请先选择卡组'); return; }
      const deck = await WailsBridge.loadDeck(pickedDeck);
      if (!deck) { setErrMsg('卡组加载失败'); return; }
      WailsBridge.updateDeck([...deck.main, ...deck.extra], deck.side);
    }
    setIsReady(next);
    WailsBridge.setReady(next);
  };

  // 开始决斗使能（duelclient.cpp:916-923）：双方都准备才可开始
  const canStart = isHost && !!seats[0].ready && !!seats[1].ready;

  useEffect(() => {
    if (connected || inRoom) {
      WailsBridge.listDecks().then((list: string[]) => {
        setDecks(list || []);
        setPickedDeck((prev: string) => prev || (list && list[0]) || '');
      }).catch(() => setDecks([]));
    }
  }, [connected, inRoom]);

  const sendChat = (): void => {
    const msg = chat.trim();
    if (!msg) return;
    WailsBridge.sendChat(msg);
    setChat('');
  };

  // 波 H：座位行改原版 wHostPrepare 形态（名字行 + X 踢人 + 准备✓只读标记）
  const seatRow = (pos: number) => {
    const seat = seats[pos];
    const isSelf = pos === selfType && !isObserver;
    const kickable = isHost && !isObserver && !!seat.name && !isSelf && inRoom;
    return (
      <div style={{ display: 'flex', alignItems: 'center', height: '25px', gap: '6px', fontSize: '13px' }}>
        {kickable ? (
          <button
            id={`lobby-kick-p${pos + 1}`}
            title="踢出该玩家"
            className="gfw-btn btn"
            style={{ width: '22px', height: '20px', padding: 0, fontSize: '11px' }}
            onClick={() => kickSeat(pos)}
          >
            X
          </button>
        ) : <span style={{ width: '22px' }} />}
        <span style={{ flex: 1, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' }}>
          {seat.name || '（空位）'}
          {isSelf ? '（你）' : ''}
          {seat.name ? <span style={{ fontSize: '11px', color: seat.color, marginLeft: '6px' }}>{seat.status}</span> : null}
        </span>
        <span
          style={{
            width: '14px', textAlign: 'center', fontWeight: 'bold',
            color: seat.ready ? '#1a7a1a' : '#b0b0b0',
          }}
          title={seat.ready ? '已准备' : '未准备'}
        >
          {seat.ready ? '✓' : ''}
        </span>
      </div>
    );
  };

  // ---- 波 H：三个原版窗口显隐相位（均常驻挂载，display 切换保冒烟 offsetParent 语义） ----
  const showLan = !connected && !inRoom && !createOpen;
  const showCreate = createOpen;
  const showPrepare = (connected || inRoom) && !createOpen;

  // 建房窗行布局（原版 label 左、控件右，game.cpp:230-300）
  const row = (label: string, control: React.ReactNode) => (
    <div style={{ display: 'flex', alignItems: 'center', height: '30px', gap: '8px' }}>
      <span className="gfw-label" style={{ width: '88px', flexShrink: 0 }}>{label}</span>
      {control}
    </div>
  );

  return (
    <div id="lobby-screen" className="screen active">
      {/* ---- wLanWindow 联机模式（580×420，game.cpp:208-228）---- */}
      <div
        id="server-connect-panel"
        className="gfw-window gfw-lan"
        style={{ display: showLan ? 'block' : 'none' }}
      >
        <div className="gfw-title">联机模式</div>
        <button
          className="gfw-btn btn"
          style={{ position: 'absolute', top: '26px', right: '10px', width: '110px' }}
          onClick={() => setCreateOpen(true)}
        >
          建立主机
        </button>
        <div className="gfw-body" style={{ padding: '8px 10px' }}>
          <div style={{ display: 'flex', alignItems: 'center', height: '30px', gap: '8px' }}>
            <span className="gfw-label" style={{ width: '70px' }}>昵称：</span>
            <input
              id="lobby-nickname"
              className="gfw-input form-input"
              type="text"
              style={{ flex: 1 }}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>
          {/* 房间列表（原版 lstHostList；本地服务器暂无房间发现协议，占位） */}
          <div id="lobby-host-list" className="gfw-list" style={{ height: '180px', margin: '6px 0' }}>
            <div className="gfw-list-item" style={{ color: '#888' }}>（本地服务器：直接用下方「启动服务器」后加入或建立主机）</div>
          </div>
          <div style={{ display: 'flex', justifyContent: 'center', margin: '4px 0 10px 0' }}>
            <button className="gfw-btn btn" style={{ width: '100px' }} onClick={() => { /* 原版刷新主机列表；本地无发现协议 */ }}>
              刷新主机
            </button>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', height: '30px', gap: '8px' }}>
            <span className="gfw-label" style={{ width: '70px' }}>主机信息：</span>
            <input
              id="lobby-join-host"
              className="gfw-input form-input"
              type="text"
              style={{ flex: 1 }}
              value={joinHost}
              onChange={(e) => setJoinHost(e.target.value)}
            />
            <input
              id="lobby-join-port"
              className="gfw-input form-input"
              type="text"
              style={{ width: '80px' }}
              value={joinPort}
              onChange={(e) => setJoinPort(e.target.value)}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', height: '30px', gap: '8px' }}>
            <span className="gfw-label" style={{ width: '70px' }}>主机密码：</span>
            <input
              id="lobby-host-pass"
              className="gfw-input form-input"
              type="text"
              style={{ flex: 1 }}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '8px' }}>
            <button className="gfw-btn btn" style={{ width: '110px' }} onClick={connectServer}>加入游戏</button>
            <button className="gfw-btn btn" style={{ width: '110px' }} onClick={() => onNavigate('menu')}>取消</button>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '14px', borderTop: '1px solid #b8b5ae', paddingTop: '10px' }}>
            <span className="gfw-label" style={{ color: '#555' }}>本地服务器端口：</span>
            <input
              id="lobby-local-port"
              className="gfw-input form-input"
              type="number"
              style={{ width: '80px' }}
              value={localPort}
              onChange={(e) => setLocalPort(e.target.value)}
            />
            <button className="gfw-btn btn" onClick={startLocalServer}>启动服务器</button>
          </div>
        </div>
      </div>

      {/* ---- wCreateHost 建立主机（380×420，game.cpp:229-301）---- */}
      <div
        className="gfw-window gfw-create room-create-panel"
        style={{ display: showCreate ? 'block' : 'none' }}
      >
        <div className="gfw-title">建立主机</div>
        <div className="gfw-body" style={{ padding: '8px 12px' }}>
          {row('禁限卡表：', (
            <select
              id="lobby-lflist-select"
              className="gfw-select form-select"
              style={{ flex: 1 }}
              value={lfLists.some((l) => l.hash === lflist) ? lflist : (lfLists[0]?.hash ?? 0)}
              onChange={(e) => setLflist(Number(e.target.value))}
            >
              {lfLists.map((l) => <option key={l.hash} value={l.hash}>{l.name}</option>)}
            </select>
          ))}
          {row('卡片允许：', (
            <select
              id="lobby-rule-select"
              className="gfw-select form-select"
              style={{ flex: 1 }}
              value={cardRule}
              onChange={(e) => setCardRule(parseInt(e.target.value, 10))}
            >
              {CARD_RULES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
            </select>
          ))}
          {row('决斗模式：', (
            <select
              id="lobby-duel-mode-select"
              className="gfw-select form-select"
              style={{ flex: 1 }}
              value={duelMode}
              onChange={(e) => setDuelMode(parseInt(e.target.value, 10))}
            >
              {DUEL_MODES.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
          ))}
          {row('每回合时间：', (
            <input
              id="lobby-timelimit"
              className="gfw-input form-input"
              type="number"
              style={{ width: '80px', textAlign: 'center' }}
              value={timeLimit}
              onChange={(e) => setTimeLimit(e.target.value)}
            />
          ))}
          {row('规则：', (
            <select
              id="lobby-duel-rule-select"
              className="gfw-select form-select"
              style={{ flex: 1 }}
              value={duelRule}
              onChange={(e) => setDuelRule(parseInt(e.target.value, 10))}
            >
              {DUEL_RULES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
            </select>
          ))}
          <div style={{ display: 'flex', gap: '16px', height: '26px', alignItems: 'center' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer' }}>
              <input id="lobby-nocheck" type="checkbox" className="gfw-check" checked={noCheckDeck} onChange={(e) => setNoCheckDeck(e.target.checked)} />
              不检查卡组
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer' }}>
              <input id="lobby-noshuffle" type="checkbox" className="gfw-check" checked={noShuffleDeck} onChange={(e) => setNoShuffleDeck(e.target.checked)} />
              不洗切卡组
            </label>
          </div>
          {row('初始基本分：', (
            <input id="lobby-startlp" className="gfw-input form-input" type="number" style={{ width: '80px', textAlign: 'center' }} value={startLp} onChange={(e) => setStartLp(e.target.value)} />
          ))}
          {row('初始手卡数：', (
            <input id="lobby-starthand" className="gfw-input form-input" type="number" style={{ width: '80px', textAlign: 'center' }} value={startHand} onChange={(e) => setStartHand(e.target.value)} />
          ))}
          {row('每回合抽卡：', (
            <input id="lobby-drawcount" className="gfw-input form-input" type="number" style={{ width: '80px', textAlign: 'center' }} value={drawCount} onChange={(e) => setDrawCount(e.target.value)} />
          ))}
          {row('房间名：', (
            <input id="lobby-room-name" className="gfw-input form-input" type="text" style={{ flex: 1 }} value={roomName} onChange={(e) => setRoomName(e.target.value)} />
          ))}
          {row('密码：', (
            <input id="lobby-room-pass" className="gfw-input form-input" type="text" style={{ flex: 1 }} value={roomPass} onChange={(e) => setRoomPass(e.target.value)} />
          ))}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '10px' }}>
            <button className="gfw-btn btn" style={{ width: '110px' }} onClick={createRoom}>创建房间</button>
            <button className="gfw-btn btn" style={{ width: '110px' }} onClick={() => setCreateOpen(false)}>取消</button>
          </div>
        </div>
      </div>

      {/* ---- wHostPrepare 决斗准备（480×320，game.cpp:304-330；右侧 stHostPrepRule 规则面板）---- */}
      <div
        id="room-lobby-panel"
        className="gfw-window gfw-prepare"
        style={{ display: showPrepare ? 'block' : 'none' }}
      >
        <div className="gfw-title">决斗准备</div>
        {/* 规则信息面板（stHostPrepRule，duelclient.cpp:453-490）：建房/加入后展示服务端下发的 HostInfo */}
        {roomRuleInfo ? (
          <pre
            id="lobby-room-rule"
            style={{
              position: 'absolute', top: '34px', right: '10px', width: '180px', height: '200px',
              overflow: 'auto', margin: 0, padding: '6px', boxSizing: 'border-box',
              fontSize: '11px', lineHeight: 1.5, color: '#16161e', whiteSpace: 'pre-wrap',
              background: '#ffffff', border: '1px solid #8a8a8a',
            }}
          >
            {roomRuleLines(roomRuleInfo, lfLists).join('\n')}
          </pre>
        ) : null}
        <div className="gfw-body" style={{ padding: '8px 10px', width: '280px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '2px', marginBottom: '8px' }}>
            {seatRow(0)}
            {seatRow(1)}
          </div>
          {/* 加入房间：仅在未建房/未加入时可用 */}
          <div style={{ display: inRoom ? 'none' : 'flex', gap: '6px', height: '30px', alignItems: 'center', margin: '4px 0' }}>
            <input
              id="lobby-join-pass"
              className="gfw-input form-input"
              type="text"
              style={{ flex: 1 }}
              placeholder="房间密码（可选）"
              value={joinPass}
              onChange={(e) => setJoinPass(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') joinRoom(); }}
            />
            <button id="lobby-join-btn" className="gfw-btn btn" style={{ width: '90px' }} onClick={joinRoom}>加入房间</button>
          </div>
          <div style={{ fontSize: '12px', margin: '2px 0' }}>
            当前观战人数：<span id="lobby-watch-count">{watchCount}</span>
          </div>
          {isObserver ? (
            <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
              <button id="lobby-to-duelist" className="gfw-btn btn" style={{ flex: 1 }} onClick={backToDuelist}>
                转为决斗者
              </button>
              <button className="gfw-btn btn" style={{ flex: 1 }} onClick={leaveRoom}>离开房间</button>
            </div>
          ) : (
            <>
              <div style={{ display: 'flex', alignItems: 'center', height: '26px', gap: '6px' }}>
                <span className="gfw-label">卡组选择：</span>
                <select
                  id="lobby-deck-select"
                  className="gfw-select form-select"
                  style={{ flex: 1 }}
                  value={pickedDeck}
                  onChange={(e) => {
                    setPickedDeck(e.target.value);
                    settingsStore.set('lastdeck', e.target.value);
                  }}
                >
                  {decks.map((d) => <option key={d} value={d}>{d}</option>)}
                </select>
              </div>
              <div style={{ display: 'flex', gap: '6px', marginTop: '6px' }}>
                <button className="gfw-btn btn" style={{ flex: 1 }} onClick={toggleReady}>
                  {isReady ? '取消准备' : '准备'}
                </button>
              {!isHost && (
                <button id="lobby-watch-btn" className="gfw-btn btn" style={{ width: '80px' }} onClick={watchAsObserver}>
                  →观战
                </button>
              )}
              {isHost && (
                <button
                  className="gfw-btn btn"
                  style={{ width: '80px' }}
                  disabled={!canStart}
                  title={canStart ? '' : '双方都准备后才能开始'}
                  onClick={() => WailsBridge.startDuel()}
                >
                  开始
                </button>
              )}
              <button className="gfw-btn btn" style={{ width: '80px' }} onClick={leaveRoom}>离开房间</button>
              </div>
            </>
          )}
        </div>
      </div>

      {/* ---- 聊天（原版 wChat 底部条；随等候区窗显隐）---- */}
      <div
        id="lobby-chat-panel"
        style={{
          display: showPrepare ? 'flex' : 'none',
          position: 'absolute', left: '50%', transform: 'translateX(-50%)',
          top: 'calc(50% + 175px)', width: '480px', height: '110px',
          flexDirection: 'column', background: '#d6d3ce', border: '1px solid #4a4a52',
          padding: '6px', boxSizing: 'border-box', zIndex: 20,
        }}
      >
        <div ref={chatBoxRef} style={{ flex: 1, overflowY: 'auto', fontSize: '12px', color: '#16161e', marginBottom: '6px', background: '#fff', border: '1px solid #8a8a8a', padding: '4px' }}>
          {messages.map((m, i) => <div key={i}>{m}</div>)}
        </div>
        <div style={{ display: 'flex', gap: '6px' }}>
          <input className="gfw-input form-input" type="text" style={{ flex: 1 }} placeholder="输入消息..." value={chat} onChange={(e) => setChat(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') sendChat(); }} />
          <button className="gfw-btn btn" style={{ width: '70px' }} onClick={sendChat}>发送</button>
        </div>
      </div>

      {/* 错误弹窗（原版 wMessage 消息窗，duelclient.cpp:261-368 的 sysString 文案） */}
      {errMsg && (
        <div
          style={{
            position: 'fixed', inset: 0, zIndex: 2000, background: 'rgba(0,0,0,0.6)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
          onClick={() => setErrMsg(null)}
        >
          <div
            className="gfw-window"
            style={{ position: 'static', transform: 'none', width: '350px' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="gfw-title">消息</div>
            <div className="gfw-body" style={{ padding: '16px' }}>
              <div style={{ fontSize: '13px', lineHeight: 1.6, marginBottom: '14px' }}>{errMsg}</div>
              <div style={{ textAlign: 'center' }}>
                <button className="gfw-btn btn" style={{ width: '80px' }} onClick={() => setErrMsg(null)}>确定</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
