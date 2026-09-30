import React, { useEffect, useRef, useState } from 'react';
import { WailsBridge, eventBus } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';
import { setLastSentDeck } from '../duel/side_deck_state.ts';
import GfwSelect from './ui/GfwSelect.tsx';
import UiButton from '../ui/UiButton.tsx';
import { UiInput, UiCheckbox } from '../ui/UiInput.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';

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
// 原版 wHostPrepare 固定创建 4 个座位行（game.cpp:308-317），非 TAG 模式只
// 用前 2 个；TAG 模式（Mode=2）4 个全用，1/2 号位为 0/1 号位的队友。
const MAX_SEATS = 4;

// LAN 广播发现到的一台主机（协议 HostPacket，duelclient.cpp BroadcastReply）
interface HostEntry {
  ip: string;
  port: number;
  name: string;
  lflist: number;
  rule: number;
  mode: number;
  duelRule: number;
  startLp: number;
  startHand: number;
  drawCount: number;
  timeLimit: number;
  noCheckDeck: number;
  noShuffleDeck: number;
}

// 建房参数取值与 vendored ygopro 对齐（game.cpp wCreateHost cbDuelRule，
// 选项下标+1 即 duel_rule；SysString 1260-1264）：
//   duelRule: 1=大师规则 / 2=大师规则２ / 3=大师规则３ /
//             4=新大师规则(2017) / 5=大师规则(2020)
//   服务端按 DuelRule<<16 透传给 ocgcore（ocgapi.cpp 直接采用该值，
//   MR1/MR2 由 processor.cpp 的 duel_rule<=2 分支支持）。
//   mode:     network.h MODE_SINGLE=0 / MODE_MATCH=1 / MODE_TAG=2
const DUEL_RULES: { value: number; label: string }[] = [
  { value: 5, label: '大师规则（2020）' },
  { value: 4, label: '新大师规则（2017）' },
  { value: 3, label: '大师规则３' },
  { value: 2, label: '大师规则２' },
  { value: 1, label: '大师规则' },
];

const DUEL_MODES: { value: number; label: string }[] = [
  { value: 0, label: '单局模式' },
  { value: 1, label: '比赛模式' },
  { value: 2, label: 'ＴＡＧ' },
];

// 卡片允许（SysString 1481+rule，game.cpp wCreateHost cbRule 六项）：
// 服务端 deck_manager.go ruleMap = {OCG, TCG, SC, CUSTOM, OCGTCG, 0}
const CARD_RULES: { value: number; label: string }[] = [
  { value: 0, label: 'ＯＣＧ' },
  { value: 1, label: 'ＴＣＧ' },
  { value: 2, label: '简体中文' },
  { value: 3, label: '自定义卡片' },
  { value: 4, label: '无独有卡' },
  { value: 5, label: '所有卡片' },
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
// 7=观战者（NETPLAYER_TYPE_OBSERVER）、8=系统、10-19 观战频道）。
// TAG 队色：0/2 一队（蓝）、1/3 一队（红），与 2 人局 0 蓝 1 红一致。
const chatIdentity = (player: number, seats: PlayerSeat[]): { name: string; color: string } => {
  if (player >= 0 && player < 4) {
    const seat = seats[player];
    if (seat && seat.name) return { name: seat.name, color: player % 2 === 0 ? '#7dd3fc' : '#fca5a5' };
    return { name: `决斗者 ${player + 1}`, color: player % 2 === 0 ? '#7dd3fc' : '#fca5a5' };
  }
  if (player === 7) return { name: '观战者', color: '#9ca3af' };
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
  // LAN 房间发现（lstHostList）：UDP 广播找房 + 点击行回填主机信息
  const [hosts, setHosts] = useState<HostEntry[]>([]);
  const [refreshingHosts, setRefreshingHosts] = useState(false);

  const [connected, setConnected] = useState(false);
  const [inRoom, setInRoom] = useState(false);
  const [isHost, setIsHost] = useState(false);
  const [selfType, setSelfType] = useState(0);
  const [isReady, setIsReady] = useState(false);
  const [decks, setDecks] = useState<string[]>([]);
  const [pickedDeck, setPickedDeck] = useState('');
  // 卡组分类（wHostPrepare cbDeckCategory：'/' 分隔的目录层级，根 = 未分类）
  const [deckCategory, setDeckCategory] = useState('');
  // 座位恒为 4 个（原版 wHostPrepare 4 座），渲染几行由房间模式决定
  const [seats, setSeats] = useState<PlayerSeat[]>([EMPTY_SEAT(), EMPTY_SEAT(), EMPTY_SEAT(), EMPTY_SEAT()]);
  const [watchCount, setWatchCount] = useState(0);
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [messages, setMessages] = useState<string[]>([]);
  const [chat, setChat] = useState('');
  // 加入流程状态行（诊断联机加入失败）：连接中→已连接→等待服务器确认→已进入/失败
  const [joinStatus, setJoinStatus] = useState('');
  const joinAckedRef = useRef(false);   // 收到 stoc:join_game 或 error_msg 即视为服务器已应答
  const joinTimerRef = useRef<number | null>(null);

  const chatBoxRef = useRef<HTMLDivElement | null>(null);
  // Observer（selftype，gframe 语义）：NETPLAYER_TYPE_OBSERVER=7（network.h:256），
  // 决斗者座位在 single/match 为 0/1、TAG 为 0-3——不能用 >1 判断观战，
  // 否则 TAG 的 2/3 号位玩家会被当成观战者（原版 duelclient 用 >3 判 TAG 观战）。
  const isObserver = selfType >= 7;
  // TAG 模式（HostInfo.Mode=2，network.h MODE_TAG）渲染 4 个座位，否则 2 个
  const roomMode = Number(roomRuleInfo?.Mode ?? roomRuleInfo?.mode ?? 0);
  const seatCount = roomMode === 2 ? MAX_SEATS : 2;

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
      if (pos < 0 || pos >= MAX_SEATS) return;
      setSeats((prev) => {
        const next = [...prev];
        next[pos] = { name: data.name || '', status: '已连接', color: '#10b981', ready: false };
        return next;
      });
    };
    // STOC_HS_PLAYER_CHANGE（duelclient.cpp:869-932）：status 低 4 位是事件、
    // 高 4 位是座位。Go 侧已拆出 pos/status。TAG 模式下座位/目标位都可能是
    // 2/3（队友位）；state<8 是座位平移（pos → state），state 8/9/a/b 分别是
    // 转观战/准备/取消准备/离开。
    const onPlayerChange = (data: any) => {
      const pos = Number(data.pos);
      const state = Number(data.status);
      setSeats((prev) => {
        const next = [...prev];
        if (state < 8) {
          // 座位平移：pos 座位的玩家移到 state 座位
          const moving = prev[pos];
          if (pos >= 0 && pos < MAX_SEATS) next[pos] = EMPTY_SEAT();
          if (state >= 0 && state < MAX_SEATS) next[state] = { ...moving, ready: false };
        } else if (state === 0x9) { // PLAYERCHANGE_READY
          if (pos >= 0 && pos < MAX_SEATS) next[pos] = { ...prev[pos], ready: true, status: '已准备', color: '#10b981' };
        } else if (state === 0xa) { // PLAYERCHANGE_NOTREADY
          if (pos >= 0 && pos < MAX_SEATS) next[pos] = { ...prev[pos], ready: false, status: '未准备', color: '#9ca3af' };
        } else if (state === 0xb) { // PLAYERCHANGE_LEAVE：清座位
          if (pos >= 0 && pos < MAX_SEATS) next[pos] = EMPTY_SEAT();
        } else if (state === 0x8) { // PLAYERCHANGE_OBSERVE：转观战，清座位
          if (pos >= 0 && pos < MAX_SEATS) next[pos] = EMPTY_SEAT();
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
        joinAckedRef.current = true;
        setJoinStatus('');
        setInRoom(false);
        setIsHost(false);
      }
    };
    const onDuelStartEvent = () => { if (onDuelStart) onDuelStart(); };
    // STOC_JOIN_GAME：HostInfo 展示在规则信息面板（duelclient.cpp:453-490）。
    // Go emit 的是 STOCJoinGame 结构体（Go 字段名直传），兼容大小写两种键
    const onJoinGame = (data: any) => {
      joinAckedRef.current = true; // 服务器已确认进房
      setJoinStatus('');
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

  // LAN 房间发现（原版 btnLanRefresh → DuelClient::BeginRefreshHost）：
  // 广播窗口打开即自动刷一轮；手动点「刷新主机」重刷（is_refreshing 守卫防重入）
  const refreshHostList = async (): Promise<void> => {
    if (refreshingHosts) return;
    setRefreshingHosts(true);
    try {
      const list = await WailsBridge.refreshHosts(3000);
      setHosts(list || []);
    } catch {
      setHosts([]);
    } finally {
      setRefreshingHosts(false);
    }
  };

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

  // 连接并直接加入房间（原版 ygopro「加入游戏」一键语义：Connect +
  // CTOS_JOIN_GAME）。密码错误等失败由服务端 STOC_ERROR_MSG 弹窗呈现。
  const connectServer = async (): Promise<void> => {
    const addr = `${joinHost.trim() || '127.0.0.1'}:${joinPort.trim() || '7911'}`;
    setJoinStatus(`正在连接 ${addr} …`);
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
      beginJoinWatch();
      const join = await WailsBridge.joinGame(password.trim());
      if (join.success) {
        setInRoom(true);
        watchJoinAck();
      } else {
        setJoinStatus('');
        setErrMsg(`加入房间失败：${join.error || '未知错误'}`);
      }
    } else {
      setJoinStatus('');
      setErrMsg(`连接失败：${res.error}`);
    }
  };

  // join 包发出后等服务器 STOC_JOIN_GAME 确认；5 秒无应答给明文提示
  // （error_msg 到达也会置 joinAcked，见 onErrorMsg）。
  // 注意时序：joinGame 的 await 尚未返回时服务端确认事件可能已先到
  // （readLoop 独立 goroutine），所以 acked 复位必须在发包之前做，
  // 发包后仅在仍未确认时才进入等待提示。
  const beginJoinWatch = (): void => {
    joinAckedRef.current = false;
    setJoinStatus('已连接，正在加入房间…');
  };
  const watchJoinAck = (): void => {
    if (joinAckedRef.current) return; // 确认事件已先到，无需等待提示
    setJoinStatus('已发送加入请求，等待服务器确认…');
    if (joinTimerRef.current) window.clearTimeout(joinTimerRef.current);
    joinTimerRef.current = window.setTimeout(() => {
      if (!joinAckedRef.current) {
        setJoinStatus('服务器 5 秒未确认加入：请检查主机地址/端口是否正确、密码是否与房主一致（详见程序目录 debug_net.log）');
      }
    }, 5000);
  };

  const startLocalServer = async (): Promise<void> => {
    const port = parseInt(localPort, 10) || 7911;
    const res = await WailsBridge.startLocalServer(port);
    if (res.success) {
      setJoinHost('127.0.0.1');
      setJoinPort(String(port));
      settingsStore.set('serverport', port);
    } else {
      setErrMsg(`启动本地服务器失败：${res.error}`);
    }
  };

  // 建立主机（原版 wCreateHost 语义：本机开服并自己入座）。未连接时自动
  // 完成「连接本地服务器（没在跑就先启动）」再建房，失败弹错误。
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
    if (!connected) {
      const port = parseInt(localPort, 10) || 7911;
      const addr = `127.0.0.1:${port}`;
      let conn = await WailsBridge.connectServer(addr, username.trim() || 'Duelist', '');
      if (!conn.success) {
        // 本地服务器没在跑：先启动再连
        const srv = await WailsBridge.startLocalServer(port);
        if (!srv.success) { setErrMsg(`启动本地服务器失败：${srv.error}`); return; }
        settingsStore.set('serverport', port);
        conn = await WailsBridge.connectServer(addr, username.trim() || 'Duelist', '');
        if (!conn.success) { setErrMsg(`连接本地服务器失败：${conn.error}`); return; }
      }
      setConnected(true);
      setSeats((prev) => {
        const next = [...prev];
        next[0] = { ...next[0], name: username.trim() || 'Duelist' };
        return next;
      });
      settingsStore.set('nickname', username.trim() || 'Duelist');
    }
    const res = await WailsBridge.createGame(req, roomName.trim() || 'Duel Room', roomPass.trim());
    if (res.success) {
      setIsHost(true);
      setInRoom(true);
      setCreateOpen(false); // 建房成功 → 进等候区窗
      settingsStore.set('gamename', roomName.trim() || 'Duel Room');
    } else {
      setErrMsg(`建立主机失败：${res.error}`);
    }
  };

  // 加入房间：CTOS_JOIN_GAME（gameID 由服务端房间列表决定，本地单房间用 0）
  const joinRoom = async (): Promise<void> => {
    beginJoinWatch();
    const res = await WailsBridge.joinGame(joinPass.trim());
    if (res.success) {
      setInRoom(true);
      watchJoinAck();
    } else if (res.error) setErrMsg(`加入房间失败：${res.error}`);
    // 密码错误等失败时服务端回 STOC_ERROR_MSG（JOINERROR），由 error_msg 弹窗呈现
  };

  // 离开房间：CTOS_LEAVE_GAME，重置等候区状态（连接保留，可随即再加入）
  const leaveRoom = (): void => {
    WailsBridge.leaveGame();
    setInRoom(false);
    setIsHost(false);
    setIsReady(false);
    setRoomRuleInfo(null);
    setJoinStatus('');
    joinAckedRef.current = true; // 阻止等待中的超时提示
    setSeats([EMPTY_SEAT(), EMPTY_SEAT(), EMPTY_SEAT(), EMPTY_SEAT()]);
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
      setLastSentDeck(deck);
      // await 保证 UPDATE_DECK 先于 HS_READY 到达服务端（乱序会被按空卡组拒绝）
      await WailsBridge.updateDeck([...deck.main, ...deck.extra], deck.side);
    }
    setIsReady(next);
    WailsBridge.setReady(next);
  };

  // 开始决斗使能（duelclient.cpp:916-923 + tag_duel.cpp 对应段）：全部座位
  // 就绪才可开始（single/match 2 席、TAG 4 席；服务端 startDuelCommon 同样校验）
  const canStart = isHost && seats.slice(0, seatCount).every((s) => s.ready);

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

  // 座位行（docs 原型 lobby-player-row：X 踢人 18×18 + 昵称输入框 20px + 准备勾选框 14×14）
  // 座位行（wHostPrepare 原版坐标 game.cpp:308-319）：0/1 号位 y65/90、2/3 号位
  // y125/150（窗口相对，内容区 = y-24）；踢人钮 (10,·) 20×20、昵称框 (40,·) 200×20、
  // 准备勾选框 (250,·) 20×20。昵称框保持 height/width 20px/200px 内联样式
  //（lobby/netplay 冒烟的 seatTexts 选择器契约）。
  const seatRow = (pos: number) => {
    const seat = seats[pos];
    const isSelf = pos === selfType && !isObserver;
    const kickable = isHost && !isObserver && !!seat.name && !isSelf && inRoom;
    const top = (pos < 2 ? 65 + pos * 25 : 75 + pos * 25) - 24;
    return (
      <div key={pos}>
        {kickable ? (
          <UiButton
            id={`lobby-kick-p${pos + 1}`}
            title="踢出该玩家"
            style={{ position: 'absolute', left: '10px', top: `${top}px`, width: '20px', height: '20px', padding: 0, fontSize: '10px' }}
            onClick={() => kickSeat(pos)}
          >
            X
          </UiButton>
        ) : null}
        <span
          style={{
            position: 'absolute', left: '40px', top: `${top}px`, width: '200px',
            height: '20px', lineHeight: '20px', background: 'rgba(8, 12, 26, 0.85)',
            border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '4px',
            color: '#e6eeff', fontSize: '12px', padding: '0 4px',
            boxSizing: 'border-box', overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis',
          }}
        >
          {seat.name || '（空位）'}{isSelf ? '（你）' : ''}
          {seat.name ? <span style={{ fontSize: '10px', color: seat.color, marginLeft: '4px' }}>{seat.status}</span> : null}
        </span>
        <span
          title={seat.ready ? '已准备' : '未准备'}
          style={{
            position: 'absolute', left: '250px', top: `${top}px`, width: '20px', height: '20px',
            border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '3px', boxSizing: 'border-box',
            background: seat.ready ? 'rgba(0, 150, 220, 0.85)' : 'rgba(8, 12, 26, 0.85)', color: '#fff',
            fontSize: '10px', lineHeight: '18px', textAlign: 'center',
          }}
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

  // LAN 窗打开时自动发现一轮房间
  useEffect(() => {
    if (showLan) refreshHostList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showLan]);

  // 主机行文案（原版 BroadcastReply 的 hoststr 拼接）：
  // [卡表][规则][模式][标准/自定义]房间名
  const hostRowLabel = (h: HostEntry): string => {
    const lf = lfNameOf(h.lflist, lfLists);
    const rule = (CARD_RULES[h.rule] || CARD_RULES[0]).label;
    const mode = (DUEL_MODES[h.mode] || DUEL_MODES[0]).label;
    // 原版：draw1/hand5/lp8000/不检查关/不洗关/默认规则(=5) → 1247「标准」
    const standard = h.drawCount === 1 && h.startHand === 5 && h.startLp === 8000
      && !h.noCheckDeck && !h.noShuffleDeck && h.duelRule === 5;
    return `[${lf}][${rule}][${mode}][${standard ? '标准' : '自定义'}]${h.name}`;
  };

  // 选中主机行 → 回填主机信息（原版 LISTBOX_LAN_HOST，menu_handler.cpp:507-519）
  const pickHost = (h: HostEntry): void => {
    setJoinHost(h.ip);
    setJoinPort(String(h.port));
  };

  // 建房窗行模板（wCreateHost 原版坐标 game.cpp:232-302）：label (20,·) 起宽 120、
  // 控件 x140 起（body 左 padding 10 + label cell paddingLeft 10 凑出 x20/x140）
  const row = (label: string, control: React.ReactNode, mt = 5) => (
    <div style={{ display: 'flex', alignItems: 'center', marginTop: `${mt}px`, height: '25px', flexShrink: 0 }}>
      <span className="gfw-label" style={{ width: '130px', flexShrink: 0, paddingLeft: '10px', boxSizing: 'border-box' }}>{label}</span>
      {control}
    </div>
  );

  // 卡组分类（'/' 目录层级；'' = 未分类）与分类内卡组（wHostPrepare 两下拉）
  const deckCategories = ['', ...[...new Set(decks.filter((d) => d.includes('/')).map((d) => d.split('/')[0]))]];
  const decksInCategory = deckCategory === ''
    ? decks.filter((d) => !d.includes('/'))
    : decks.filter((d) => d.startsWith(deckCategory + '/'));
  const onCategoryChange = (cat: string): void => {
    setDeckCategory(cat);
    settingsStore.set('lastcategory', cat);
    const inCat = cat === '' ? decks.filter((d) => !d.includes('/')) : decks.filter((d) => d.startsWith(cat + '/'));
    if (!inCat.includes(pickedDeck)) setPickedDeck(inCat[0] || '');
  };

  return (
    <div id="lobby-screen" className="screen active">
      <DesignSpace>
      {/* ---- wLanWindow 联机模式（game.cpp:208-227）：窗口 (220,100)-(800,520) 580×420；
              内部坐标（窗口相对）：昵称行 y25-50（ebNickName 110-450 + btnCreateHost 460-570）、
              lstHostList (10,60)-(570,320)、btnLanRefresh (240,325)-(340,350)、
              主机信息行 y355-380（ebJoinHost 110-350 + ebJoinPort 360-420 + btnJoinHost 460-570）、
              主机密码行 y385-410（ebJoinPass 110-420 + btnJoinCancel 460-570） ---- */}
      <div
        id="server-connect-panel"
        className="gfw-window gfw-lan"
        style={{ display: showLan ? 'block' : 'none' }}
      >
        <div className="gfw-title">联机模式</div>
        {/* 内容区 = 420-24 标题 = 396px；行高/间距按上方原版坐标折算 */}
        <div style={{ height: '396px', padding: '0 10px 10px', boxSizing: 'border-box', display: 'flex', flexDirection: 'column' }}>
          {/* 昵称行：label(10,30) + ebNickName(110,25)-(450,50) + btnCreateHost(460,25)-(570,50) */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginTop: '1px', height: '25px', flexShrink: 0 }}>
            <span className="gfw-label" style={{ width: '100px', flexShrink: 0 }}>昵称：</span>
            <UiInput
              id="lobby-nickname"
             
              type="text"
              style={{ flex: 1 }}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <UiButton
             
              style={{ width: '110px', flexShrink: 0 }}
              onClick={() => setCreateOpen(true)}
            >
              建立主机
            </UiButton>
          </div>
          {/* lstHostList (10,60)-(570,320) = 560×260（LAN UDP 广播发现，点击行回填主机信息） */}
          <div id="lobby-host-list" className="gfw-list" style={{ marginTop: '10px', height: '260px', flexShrink: 0, overflowY: 'auto' }}>
            {hosts.length === 0 ? (
              <div className="gfw-list-item" style={{ color: '#7b89ab' }}>
                {refreshingHosts ? '正在搜索局域网主机……' : '未发现主机——可「建立主机」本机开服，或输入主机信息加入游戏'}
              </div>
            ) : hosts.map((h, i) => (
              <div
                key={`${h.ip}:${h.port}`}
                id={`lobby-host-${i}`}
                className="gfw-list-item"
                style={{ cursor: 'pointer' }}
                title="点击回填主机信息"
                onClick={() => pickHost(h)}
              >
                {hostRowLabel(h)}
              </div>
            ))}
          </div>
          {/* btnLanRefresh (240,325)-(340,350) 居中；右侧空区放本地服务器扩展行
              （原版无此控件：本地无发现协议，保留功能入口） */}
          <div style={{ marginTop: '5px', height: '25px', flexShrink: 0, display: 'flex', alignItems: 'center' }}>
            <div style={{ flex: 1 }} />
            <UiButton
              id="lobby-refresh-hosts"
             
              style={{ width: '100px' }}
              disabled={refreshingHosts}
              onClick={refreshHostList}
            >
              {refreshingHosts ? '刷新中…' : '刷新主机'}
            </UiButton>
            <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '4px' }}>
              <span className="gfw-label">本地端口：</span>
              <UiInput
                id="lobby-local-port"
               
                type="number"
                style={{ width: '50px', flexShrink: 0 }}
                value={localPort}
                onChange={(e) => setLocalPort(e.target.value)}
              />
              <UiButton style={{ width: '90px', flexShrink: 0 }} onClick={startLocalServer}>启动服务器</UiButton>
            </div>
          </div>
          {/* 底部：左列 x10-420（主机信息行 y355-380 / 主机密码行 y385-410），
              右列 btnJoinHost (460,355)-(570,380) / btnJoinCancel (460,385)-(570,410) 110×25 */}
          <div style={{ marginTop: '5px', flexShrink: 0, display: 'flex' }}>
            <div style={{ width: '410px', flexShrink: 0, display: 'flex', flexDirection: 'column', gap: '5px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '25px' }}>
                <span className="gfw-label" style={{ width: '100px', flexShrink: 0 }}>主机信息：</span>
                <UiInput
                  id="lobby-join-host"
                 
                  type="text"
                  style={{ flex: 1 }}
                  value={joinHost}
                  onChange={(e) => setJoinHost(e.target.value)}
                />
                <UiInput
                  id="lobby-join-port"
                 
                  type="text"
                  style={{ width: '60px', flexShrink: 0 }}
                  value={joinPort}
                  onChange={(e) => setJoinPort(e.target.value)}
                />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', height: '25px' }}>
                <span className="gfw-label" style={{ width: '100px', flexShrink: 0 }}>主机密码：</span>
                <UiInput
                  id="lobby-host-pass"
                 
                  type="text"
                  style={{ flex: 1 }}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>
            <div style={{ flex: 1 }} />
            <div style={{ width: '110px', flexShrink: 0, display: 'flex', flexDirection: 'column', gap: '5px' }}>
              <UiButton onClick={connectServer}>加入游戏</UiButton>
              <UiButton onClick={() => onNavigate('menu')}>取消</UiButton>
            </div>
          </div>
        </div>
      </div>

      {/* ---- wCreateHost 建立主机（game.cpp:229-302）：窗口 (320,100)-(700,520) 380×420 ---- */}
      <div
        className="gfw-window gfw-create room-create-panel"
        style={{ display: showCreate ? 'flex' : 'none', flexDirection: 'column' }}
      >
        <div className="gfw-title">建立主机</div>
        {/* 内容区 396px（420-24 标题），行坐标对齐原版（窗口相对 y-24）：
            禁限卡表 y25 / 卡片允许 y55 / 决斗模式 y85 / 每回合时间 y115（输入框 140-220）/
            提示行 y150-170 / 规则 y175 / 复选框 y210-230 / 初始三项 y235/265/295 /
            底部 主机名称 y355 + 主机密码 y385（输入框 110-250），确定/取消 (260,355/385) 110×25 */}
        <div style={{ height: '396px', padding: '0 10px 10px', boxSizing: 'border-box', display: 'flex', flexDirection: 'column' }}>
          {row('禁限卡表：', (
            <GfwSelect
              id="lobby-lflist-select"
              width={160}
              // Radix 受控值必须落在 options 内：空表时给 'N/A' 占位项
              //（对应原生下拉的 lfLists[0]?.hash ?? 0 语义）
              value={String(lfLists.some((l) => l.hash === lflist) ? lflist : (lfLists[0]?.hash ?? 0))}
              onValueChange={(v) => setLflist(Number(v))}
              options={lfLists.length
                ? lfLists.map((l) => ({ value: String(l.hash), label: l.name }))
                : [{ value: '0', label: 'N/A' }]}
            />
          ), 1)}
          {row('卡片允许：', (
            <GfwSelect
              id="lobby-rule-select"
              width={160}
              value={String(cardRule)}
              onValueChange={(v) => setCardRule(parseInt(v, 10))}
              options={CARD_RULES.map((r) => ({ value: String(r.value), label: r.label }))}
            />
          ))}
          {row('决斗模式：', (
            <GfwSelect
              id="lobby-duel-mode-select"
              width={160}
              value={String(duelMode)}
              onValueChange={(v) => setDuelMode(parseInt(v, 10))}
              options={DUEL_MODES.map((m) => ({ value: String(m.value), label: m.label }))}
            />
          ))}
          {row('每回合时间：', (
            <UiInput
              id="lobby-timelimit"
             
              type="number"
              style={{ width: '80px', flexShrink: 0, textAlign: 'center' }}
              value={timeLimit}
              onChange={(e) => setTimeLimit(e.target.value)}
            />
          ))}
          {/* 原版 y150-170 的静态文本位（SysString 1228）：额外选项提示 */}
          <div style={{ marginTop: '10px', height: '20px', flexShrink: 0, fontSize: '11px', color: '#7b89ab', paddingLeft: '10px', lineHeight: '20px' }}>↓额外选项（无特殊要求请勿修改）</div>
          {row('规则：', (
            <GfwSelect
              id="lobby-duel-rule-select"
              width={160}
              value={String(duelRule)}
              onValueChange={(v) => setDuelRule(parseInt(v, 10))}
              options={DUEL_RULES.map((r) => ({ value: String(r.value), label: r.label }))}
            />
          ))}
          {/* chkNoCheckDeck (20,210)-(170,230) / chkNoShuffleDeck (180,210)-(360,230) */}
          <div style={{ marginTop: '10px', height: '20px', flexShrink: 0, display: 'flex', alignItems: 'center' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', fontSize: '12px', marginLeft: '10px', width: '150px' }}>
              <UiCheckbox id="lobby-nocheck" checked={noCheckDeck} onChange={(e) => setNoCheckDeck(e.target.checked)} />
              不检查卡组
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', fontSize: '12px', marginLeft: '10px' }}>
              <UiCheckbox id="lobby-noshuffle" checked={noShuffleDeck} onChange={(e) => setNoShuffleDeck(e.target.checked)} />
              不洗切卡组
            </label>
          </div>
          {row('初始基本分：', (
            <UiInput id="lobby-startlp" type="number" style={{ width: '80px', flexShrink: 0, textAlign: 'center' }} value={startLp} onChange={(e) => setStartLp(e.target.value)} />
          ))}
          {row('初始手卡数：', (
            <UiInput id="lobby-starthand" type="number" style={{ width: '80px', flexShrink: 0, textAlign: 'center' }} value={startHand} onChange={(e) => setStartHand(e.target.value)} />
          ))}
          {row('每回合抽卡：', (
            <UiInput id="lobby-drawcount" type="number" style={{ width: '80px', flexShrink: 0, textAlign: 'center' }} value={drawCount} onChange={(e) => setDrawCount(e.target.value)} />
          ))}
          {/* 底部（spacer 顶到 y355）：左 主机名称/主机密码（label x10 宽100 + 输入框 110-250），
              右 btnHostConfirm/btnHostCancel (260,355/385)-(370,380/410) 110×25 */}
          <div style={{ flex: 1 }} />
          <div style={{ display: 'flex', flexShrink: 0 }}>
            <div style={{ width: '240px', flexShrink: 0, display: 'flex', flexDirection: 'column', gap: '5px' }}>
              <div style={{ display: 'flex', alignItems: 'center', height: '25px' }}>
                <span className="gfw-label" style={{ width: '100px', flexShrink: 0 }}>主机名称：</span>
                <UiInput id="lobby-room-name" type="text" style={{ width: '140px', flexShrink: 0 }} value={roomName} onChange={(e) => setRoomName(e.target.value)} />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', height: '25px' }}>
                <span className="gfw-label" style={{ width: '100px', flexShrink: 0 }}>主机密码：</span>
                <UiInput id="lobby-room-pass" type="text" style={{ width: '140px', flexShrink: 0 }} value={roomPass} onChange={(e) => setRoomPass(e.target.value)} />
              </div>
            </div>
            <div style={{ flex: 1 }} />
            <div style={{ width: '110px', flexShrink: 0, display: 'flex', flexDirection: 'column', gap: '5px' }}>
              <UiButton onClick={createRoom}>确定</UiButton>
              <UiButton onClick={() => setCreateOpen(false)}>取消</UiButton>
            </div>
          </div>
        </div>
      </div>

      {/* ---- wHostPrepare 决斗准备（game.cpp:304-333）：窗口 (270,120)-(750,440) 480×320 ---- */}
      <div
        id="room-lobby-panel"
        className="gfw-window gfw-prepare"
        style={{ display: showPrepare ? 'block' : 'none' }}
      >
        <div className="gfw-title">决斗准备</div>
        {/* 内容区 296px（320-24 标题），全绝对定位对齐原版（窗口相对 y-24，game.cpp:307-333）：
            btnHostPrepDuelist (10,30) 100×25 / 座位行 y65/90/125/150 / btnHostPrepOB (10,180) /
            btnHostPrepReady (170,180) 100×25 / 卡组选择 label(10,210) + 两下拉 (10,230) 128、(142,230) 198 /
            stHostPrepRule (280,30) 180×200 / stHostPrepOB (10,285) / 开始 (230,280) 110、退出 (350,280) 110 */}
        <div style={{ position: 'relative', height: '296px' }}>
          {/* 转为决斗者（btnHostPrepDuelist，观战身份时出现） */}
          {isObserver ? (
            <UiButton id="lobby-to-duelist" style={{ position: 'absolute', left: '10px', top: '6px', width: '100px' }} onClick={backToDuelist}>转为决斗者</UiButton>
          ) : null}
          {/* 座位行：原版恒建 4 座，TAG 模式全渲染 */}
          {Array.from({ length: seatCount }, (_, i) => seatRow(i))}
          {/* 转为观战（btnHostPrepOB (10,180)-(110,205)；宿主/观战者不显示） */}
          {!isObserver && !isHost && inRoom ? (
            <UiButton id="lobby-watch-btn" style={{ position: 'absolute', left: '10px', top: '156px', width: '100px' }} onClick={watchAsObserver}>转为观战</UiButton>
          ) : null}
          {/* 准备（btnHostPrepReady/btnHostPrepNotReady (170,180)-(270,205) 100×25） */}
          {inRoom && !isObserver ? (
            <UiButton style={{ position: 'absolute', left: '170px', top: '156px', width: '100px' }} onClick={toggleReady}>
              {isReady ? '取消准备' : '准备'}
            </UiButton>
          ) : null}
          {/* 卡组选择：label (10,210)-(110,230) + cbCategorySelect (10,230)-(138,255) + cbDeckSelect (142,230)-(340,255)（观战者无卡组操作） */}
          {!isObserver ? (
            <>
              <span className="gfw-label" style={{ position: 'absolute', left: '10px', top: '186px' }}>卡组选择：</span>
              <div style={{ position: 'absolute', left: '10px', top: '206px' }}>
                <GfwSelect
                  id="lobby-deck-category"
                  width={128}
                  // Radix Item 不允许空串 value：根分类 '' 用哨兵 '__root__' 表示
                  value={deckCategory || '__root__'}
                  onValueChange={(v) => onCategoryChange(v === '__root__' ? '' : v)}
                  options={deckCategories.map((c) => ({ value: c || '__root__', label: c || '未分类卡组' }))}
                />
              </div>
              <div style={{ position: 'absolute', left: '142px', top: '206px' }}>
                <GfwSelect
                  id="lobby-deck-select"
                  width={198}
                  value={pickedDeck}
                  onValueChange={(v) => {
                    setPickedDeck(v);
                    settingsStore.set('lastdeck', v);
                  }}
                  options={decksInCategory.map((d) => ({
                    value: d,
                    label: deckCategory && d.startsWith(deckCategory + '/') ? d.slice(deckCategory.length + 1) : d,
                  }))}
                />
              </div>
            </>
          ) : null}
          {/* stHostPrepRule (280,30)-(460,230) 180×200 带边框；未进房时同区域放加入行扩展件
             （原版无加入行：连接后由 LAN 窗直接 JOIN_GAME；本地单房间流程需要入口） */}
          <div style={{
            position: 'absolute', left: '280px', top: '6px', width: '180px', height: '200px',
            border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '4px', boxSizing: 'border-box',
            background: 'rgba(8, 12, 26, 0.6)', padding: '4px 6px', overflow: 'hidden',
            fontSize: '12px', color: '#c9d6f2',
          }}>
            <div style={{ display: inRoom ? 'none' : 'flex', flexDirection: 'column', gap: '4px' }}>
              <UiInput
                id="lobby-join-pass"
               
                type="text"
                placeholder="房间密码（可选）"
                value={joinPass}
                onChange={(e) => setJoinPass(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') joinRoom(); }}
              />
              <UiButton id="lobby-join-btn" onClick={joinRoom}>加入房间</UiButton>
              {/* 已连接但不在房间时的建房入口（联机模式窗在 connected 后隐藏，
                  没有它房主连上服务器后反而无法建房） */}
              <UiButton id="lobby-create-btn" onClick={() => setCreateOpen(true)}>建立主机</UiButton>
            </div>
            {inRoom && roomRuleInfo ? (
              <div id="lobby-room-rule" style={{ whiteSpace: 'pre-wrap', lineHeight: '1.5' }}>
                {roomRuleLines(roomRuleInfo, lfLists).join('\n')}
              </div>
            ) : null}
          </div>
          {/* stHostPrepOB 观战人数 (10,285)-(270,305) */}
          <div style={{ position: 'absolute', left: '10px', top: '261px', fontSize: '12px', color: '#c9d6f2' }}>
            当前观战人数：<span id="lobby-watch-count">{watchCount}</span>
          </div>
          {/* btnHostPrepStart (230,280)-(340,305) / btnHostPrepCancel (350,280)-(460,305) 110×25 */}
          {isHost && !isObserver && (
            <UiButton
             
              style={{ position: 'absolute', left: '230px', top: '256px', width: '110px' }}
              disabled={!canStart}
              title={canStart ? '' : '所有决斗者都准备后才能开始'}
              onClick={() => WailsBridge.startDuel()}
            >
              开始
            </UiButton>
          )}
          <UiButton style={{ position: 'absolute', left: '350px', top: '256px', width: '110px' }} onClick={leaveRoom}>退出</UiButton>
        </div>
      </div>

      {/* ---- 聊天（扩展件：原版等候区无聊天窗；贴在决斗准备窗 (270,120)+320 下方，随等候区窗显隐）---- */}
      <div
        id="lobby-chat-panel"
        style={{
          display: showPrepare ? 'flex' : 'none',
          position: 'absolute', left: '270px', top: '450px', transform: 'none',
          width: '480px', height: '110px',
          flexDirection: 'column', background: 'linear-gradient(180deg, rgba(30, 39, 68, 0.97) 0%, rgba(17, 23, 44, 0.97) 100%)',
          border: '1px solid rgba(98, 132, 202, 0.45)', borderRadius: '10px',
          padding: '6px', boxSizing: 'border-box', zIndex: 20,
        }}
      >
        <div ref={chatBoxRef} style={{ flex: 1, overflowY: 'auto', fontSize: '12px', color: '#c9d6f2', marginBottom: '6px', background: 'rgba(8, 12, 26, 0.85)', border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '4px', padding: '4px' }}>
          {messages.map((m, i) => <div key={i}>{m}</div>)}
        </div>
        <div style={{ display: 'flex', gap: '6px' }}>
          <UiInput type="text" style={{ flex: 1 }} placeholder="输入消息..." value={chat} onChange={(e) => setChat(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') sendChat(); }} />
          <UiButton style={{ width: '70px' }} onClick={sendChat}>发送</UiButton>
        </div>
      </div>

      {/* 加入流程状态条（诊断联机加入）：固定在大厅屏幕底部居中 */}
      {joinStatus && (
        <div
          id="lobby-join-status"
          className="gfw-window"
          style={{
            position: 'fixed', left: '50%', bottom: '14px', transform: 'translateX(-50%)',
            padding: '6px 14px', fontSize: '12px', zIndex: 1500, whiteSpace: 'nowrap',
          }}
        >
          {joinStatus}
        </div>
      )}

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
                <UiButton id="lobby-errmsg-ok" style={{ width: '80px' }} onClick={() => setErrMsg(null)}>确定</UiButton>
              </div>
            </div>
          </div>
        </div>
      )}
      </DesignSpace>
    </div>
  );
}
