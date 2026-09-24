/**
 * 聊天浮层（原版 wChat + 聊天按钮气泡）：最近 8 条，决斗者蓝/红、
 * 观战者灰色（gframe drawing.cpp 的聊天绘制规则），12 秒后自动淡出，
 * 鼠标悬停或聚焦输入框时不消失。Enter 发送。
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { settingsStore } from '../domain/settings.ts';

const MAX_MESSAGES = 8;
const FADE_MS = 12000;

type ChatMsg = { id: number; pos: number; cls: string; name: string; text: string };

/**
 * 原版 chatColor 20 档逐槽对位表（drawing.cpp:1041 chatColor[]，chatType=玩家槽位）：
 *   0-3 决斗者（下方按 self/opp 上色，此处 null 落空）
 *   4-7 保留
 *   8   系统消息 0xff8080ff 蓝紫
 *   9-11 红 0xffff4040
 *   12 绿 / 13 蓝 / 14 青 / 15 品红 / 16 黄 / 17 白 / 18 灰 / 19 深灰
 * （duelclient.cpp STOC_CHAT：决斗者 0-3 经 ChatLocalPlayer 本地化，8 系统，
 * 10-19 观战者；观战者槽位 10-19 原版按此表逐位定色，非轮换。）
 */
const CHAT_COLOR_BY_SLOT: (string | null)[] = [
  null, null, null, null, null, null, null, null, // 0-7
  'chat-sys',         // 8
  'chat-red',         // 9
  'chat-red',         // 10
  'chat-red',         // 11
  'chat-green',       // 12
  'chat-blue',        // 13
  'chat-cyan',        // 14
  'chat-magenta',     // 15
  'chat-yellow',      // 16
  'chat-white',       // 17
  'chat-gray',        // 18
  'chat-darkgray',    // 19
];
function chatClass(player: number, selfSlot: number): string {
  const slotClass = CHAT_COLOR_BY_SLOT[player];
  if (slotClass) return slotClass;
  return player === selfSlot ? 'chat-self' : 'chat-opp';
}

export default function ChatOverlay() {
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [draft, setDraft] = useState('');
  const [hovering, setHovering] = useState(false);
  const nextId = useRef(1);

  useEffect(() => {
    const onChat = (data: { player: number; msg: string }) => {
      const playerSlot = duelStore.getState().playerSlot;
      // 身份标签（netserver.cpp:380-385）：0-3 决斗者、8 系统、10-19 观战者
      const isSpectator = data.player >= 10;
      const isSystem = data.player === 8;
      // 屏蔽聊天（原版 chkIgnore1/chkIgnore2，game.cpp:433-436）
      if (isSpectator && settingsStore.get('mute_spectators')) return;
      if (!isSpectator && !isSystem && data.player !== playerSlot && settingsStore.get('mute_opponent')) return;
      const name = isSystem ? '系统'
        : data.player === playerSlot ? '你'
        : isSpectator ? `观战者 ${data.player - 9}`
        : `玩家 ${data.player}`;
      const id = nextId.current++;
      setMessages((prev) => [...prev.slice(-(MAX_MESSAGES - 1)), {
        id,
        pos: data.player,
        cls: chatClass(data.player, playerSlot),
        name,
        text: data.msg,
      }]);
      setTimeout(() => {
        if (!hovering) {
          setMessages((prev) => prev.filter((m) => m.id !== id));
        }
      }, FADE_MS);
    };
    eventBus.on('stoc:chat', onChat);
    return () => { eventBus.off('stoc:chat', onChat); };
    // hovering 读的是闭包旧值无妨：超时只是尽力清理，悬停期间残留到下一条
    // 到来时被 slice 挤掉。
  }, []);

  const send = () => {
    const msg = draft.trim();
    if (!msg) return;
    WailsBridge.sendChat(msg);
    setDraft('');
  };

  return (
    <div
      id="chat-overlay"
      className="chat-overlay"
      onMouseEnter={() => setHovering(true)}
      onMouseLeave={() => setHovering(false)}
    >
      <div id="chat-messages" className="chat-messages">
        {messages.map((m) => (
          <div key={m.id} className={`chat-line chat-p${m.pos} ${m.cls}`}>
            <span className="chat-name">{m.name}：</span>{m.text}
          </div>
        ))}
      </div>
      <div className="chat-input-row">
        <input
          id="chat-input"
          type="text"
          placeholder="聊天..."
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onFocus={() => setHovering(true)}
          onBlur={() => setHovering(false)}
          onKeyDown={(e) => { if (e.key === 'Enter') send(); }}
        />
      </div>
    </div>
  );
}
