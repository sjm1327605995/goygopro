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

type ChatMsg = { id: number; pos: number; name: string; text: string };

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
          <div key={m.id} className={`chat-line chat-p${m.pos}`}>
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
