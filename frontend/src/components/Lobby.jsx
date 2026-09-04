import React, { useEffect, useRef, useState } from 'react';
import { WailsBridge, eventBus } from '../wails_bridge.js';

export default function Lobby({ onNavigate, onDuelStart }) {
  const [serverAddr, setServerAddr] = useState('127.0.0.1:7911');
  const [username, setUsername] = useState('YugiMuto');
  const [password, setPassword] = useState('');
  const [localPort, setLocalPort] = useState('7911');
  const [roomName, setRoomName] = useState('Championship Match');
  const [roomPass, setRoomPass] = useState('');

  const [connected, setConnected] = useState(false);
  const [isHost, setIsHost] = useState(false);
  const [isReady, setIsReady] = useState(false);
  const [p1, setP1] = useState({ name: 'YugiMuto', status: '已连接', color: '#10b981' });
  const [p2, setP2] = useState({ name: 'SetoKaiba', status: '等待玩家加入...', color: '#9ca3af' });
  const [messages, setMessages] = useState([]);
  const [chat, setChat] = useState('');

  const chatBoxRef = useRef(null);

  useEffect(() => {
    const onTypeChange = (data) => {
      setIsHost(!!data.isHost);
    };
    const onPlayerEnter = (data) => {
      if (data.pos === 0) setP1((p) => ({ ...p, name: data.name, status: '已连接', color: '#10b981' }));
      else if (data.pos === 1) setP2((p) => ({ ...p, name: data.name, status: '已连接', color: '#10b981' }));
    };
    const onPlayerChange = (data) => {
      const status = data.ready ? '已准备' : '未准备';
      const color = data.ready ? '#10b981' : '#9ca3af';
      if (data.pos === 0) setP1((p) => ({ ...p, status, color }));
      else if (data.pos === 1) setP2((p) => ({ ...p, status, color }));
    };
    const onDuelStartEvent = () => { if (onDuelStart) onDuelStart(); };
    const onChat = (data) => {
      setMessages((m) => [...m, `[决斗者 ${data.player}]: ${data.msg}`]);
    };

    eventBus.on('stoc:type_change', onTypeChange);
    eventBus.on('stoc:player_enter', onPlayerEnter);
    eventBus.on('stoc:player_change', onPlayerChange);
    eventBus.on('stoc:duel_start', onDuelStartEvent);
    eventBus.on('stoc:chat', onChat);
  }, [onDuelStart]);

  useEffect(() => {
    if (chatBoxRef.current) chatBoxRef.current.scrollTop = chatBoxRef.current.scrollHeight;
  }, [messages]);

  const connectServer = async () => {
    const res = await WailsBridge.connectServer(serverAddr.trim() || '127.0.0.1:7911', username.trim() || 'Duelist', password.trim());
    if (res.success) {
      setConnected(true);
      setP1((p) => ({ ...p, name: username.trim() || 'Duelist' }));
    } else {
      alert(`连接失败：${res.error}`);
    }
  };

  const startLocalServer = async () => {
    const port = parseInt(localPort, 10) || 7911;
    const res = await WailsBridge.startLocalServer(port);
    if (res.success) {
      setServerAddr(`127.0.0.1:${port}`);
      setConnected(true);
    } else {
      alert(`启动本地服务器失败：${res.error}`);
    }
  };

  const createRoom = async () => {
    const req = {
      lflist: 0, rule: 0, mode: 0, duelRule: 5,
      startLp: 8000, startHand: 5, drawCount: 1, timeLimit: 180,
    };
    const res = await WailsBridge.createGame(req, roomName.trim() || 'Duel Room', roomPass.trim());
    if (res.success) setIsHost(true);
  };

  const toggleReady = () => {
    const next = !isReady;
    setIsReady(next);
    WailsBridge.setReady(next);
  };

  const sendChat = () => {
    const msg = chat.trim();
    if (!msg) return;
    WailsBridge.sendChat(msg);
    setChat('');
  };

  return (
    <div id="lobby-screen" className="screen active">
      <div className="lobby-header">
        <div className="lobby-title">决斗大厅与服务器连接</div>
        <button className="btn btn-secondary" onClick={() => onNavigate('menu')}>← 返回主菜单</button>
      </div>

      <div className="lobby-content">
        <div id="server-connect-panel" className="server-panel" style={{ display: connected ? 'none' : 'block' }}>
          <h3 style={{ color: 'var(--primary)', marginBottom: '16px' }}>连接服务器</h3>
          <div className="form-group">
            <label>服务器地址 (IP:端口)</label>
            <input className="form-input" type="text" value={serverAddr} onChange={(e) => setServerAddr(e.target.value)} />
          </div>
          <div className="form-group">
            <label>决斗者昵称</label>
            <input className="form-input" type="text" value={username} onChange={(e) => setUsername(e.target.value)} />
          </div>
          <div className="form-group">
            <label>房间/密码</label>
            <input className="form-input" type="text" placeholder="可选" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>
          <div style={{ display: 'flex', gap: '12px', marginTop: '8px' }}>
            <button className="btn btn-primary" style={{ flex: 1 }} onClick={connectServer}>连接服务器</button>
          </div>

          <hr style={{ border: 0, borderTop: '1px solid var(--border-color)', margin: '24px 0' }} />

          <h3 style={{ color: 'var(--accent-gold)', marginBottom: '16px' }}>本地建主</h3>
          <div className="form-group">
            <label>本地端口</label>
            <input className="form-input" type="number" value={localPort} onChange={(e) => setLocalPort(e.target.value)} />
          </div>
          <button className="btn btn-gold" onClick={startLocalServer}>启动服务器并建房</button>
        </div>

        <div id="room-lobby-panel" className="room-lobby-panel" style={{ display: connected ? 'block' : 'none' }}>
          <h3 style={{ color: 'var(--primary)', marginBottom: '16px' }}>房间等候区</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '16px' }}>
            <div style={{ background: 'rgba(0,0,0,0.4)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>房主（玩家 1）</div>
              <div style={{ fontSize: '18px', fontWeight: 700, color: 'var(--primary)', margin: '6px 0' }}>{p1.name}</div>
              <div style={{ fontSize: '12px', color: p1.color }}>{p1.status}</div>
            </div>
            <div style={{ background: 'rgba(0,0,0,0.4)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>对手（玩家 2）</div>
              <div style={{ fontSize: '18px', fontWeight: 700, color: '#ef4444', margin: '6px 0' }}>{p2.name}</div>
              <div style={{ fontSize: '12px', color: p2.color }}>{p2.status}</div>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px', marginBottom: '16px' }}>
            <button className={isReady ? 'btn btn-secondary' : 'btn btn-gold'} style={{ flex: 1 }} onClick={toggleReady}>
              {isReady ? '取消准备' : '准备'}
            </button>
            <button
              className="btn btn-primary"
              style={{ flex: 1, display: isHost ? 'inline-flex' : 'none' }}
              onClick={() => WailsBridge.startDuel()}
            >
              开始决斗
            </button>
          </div>

          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', background: 'rgba(0,0,0,0.4)', borderRadius: '8px', padding: '12px' }}>
            <div ref={chatBoxRef} style={{ flex: 1, overflowY: 'auto', fontSize: '12px', marginBottom: '8px' }}>
              {messages.map((m, i) => <div key={i}>{m}</div>)}
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input className="form-input" type="text" placeholder="输入消息..." value={chat} onChange={(e) => setChat(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') sendChat(); }} />
              <button className="btn btn-primary" style={{ padding: '0 16px' }} onClick={sendChat}>发送</button>
            </div>
          </div>
        </div>

        <div className="room-create-panel">
          <h3 style={{ color: 'var(--text-main)', marginBottom: '16px' }}>房间设置</h3>
          <div className="form-group">
            <label>房间名</label>
            <input className="form-input" type="text" value={roomName} onChange={(e) => setRoomName(e.target.value)} />
          </div>
          <div className="form-group">
            <label>房间密码</label>
            <input className="form-input" type="text" placeholder="可选" value={roomPass} onChange={(e) => setRoomPass(e.target.value)} />
          </div>
          <div className="form-group">
            <label>决斗规则</label>
            <select className="form-select">
              <option>大师规则 2020 (MR5)</option>
              <option>疾速决斗</option>
              <option>传统/经典</option>
            </select>
          </div>
          <div className="form-group">
            <label>决斗模式</label>
            <select className="form-select">
              <option>单局赛（1 局）</option>
              <option>比赛赛（三局两胜）</option>
              <option>组队赛（2 对 2）</option>
            </select>
          </div>
          <button className="btn btn-primary" style={{ marginTop: 'auto' }} onClick={createRoom}>创建房间</button>
        </div>
      </div>
    </div>
  );
}
