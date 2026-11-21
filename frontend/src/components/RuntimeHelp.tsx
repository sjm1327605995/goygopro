import React, {useState} from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import {WailsBridge} from '../wails_bridge.ts';
import '../../css/runtime-help.css';

export default function RuntimeHelp() {
  const [open,setOpen]=useState(false);
  const [copied,setCopied]=useState(false);
  const [copyFailed,setCopyFailed]=useState(false);
  const desktop=WailsBridge.isWails;
  const commands='python tools/fetch_scripts.py\nnpm --prefix frontend ci\nnpm --prefix frontend run build\ngo run ./cmd/wails';
  const copy=async()=>{
    try { await navigator.clipboard.writeText(commands);setCopied(true);setCopyFailed(false); }
    catch {setCopied(false);setCopyFailed(true);}
  };
  return <Dialog.Root open={open} onOpenChange={value=>{setOpen(value);setCopied(false);setCopyFailed(false);}}>
    <Dialog.Trigger aria-label={`运行模式：${desktop?'桌面':'浏览器演示'}，查看帮助`} className={`runtime-mode-chip${desktop?'':' runtime-mode-chip--preview'}`}>
      <i aria-hidden="true" />{desktop?'桌面模式':'浏览器演示'}<span>运行帮助</span>
    </Dialog.Trigger>
    <Dialog.Portal><Dialog.Overlay className="runtime-help-backdrop" />
      <Dialog.Content className="runtime-help-dialog" aria-describedby="runtime-help-description">
        <span className="runtime-help-eyebrow">GOYGOPRO / RUNTIME</span>
        <Dialog.Title>{desktop?'桌面已连接':'当前是浏览器演示模式'}</Dialog.Title>
        <Dialog.Description id="runtime-help-description">{desktop?'已加载 Wails 桥接。卡库、单机脚本和卡图仍需在本机准备齐全。':'这里只提供样例数据。真实单机引擎、本地卡库和文件保存需要桌面窗口。'}</Dialog.Description>
        {!desktop&&<p className="runtime-help-note">如果你已经打开桌面窗口却仍看到此提示，请检查 Wails 运行时是否加载成功。</p>}
        <dl className="runtime-help-facts">
          <div><dt>单人残局</dt><dd>{desktop?'从 single/ 加载，运行依赖 script/ 与原生引擎。':'可以查看样例说明，不能在浏览器里执行 Lua。'}</dd></div>
          <div><dt>卡组编辑</dt><dd>{desktop?'读取 cards.cdb 和 deck/；无搜索结果时先清空筛选。':'搜索少量英文样例卡片；可试卡号 89631139。修改不会保存到本地卡组文件。'}</dd></div>
          <div><dt>卡片图片</dt><dd>pics/ 不随代码同步。图片缺失不等于卡库或残局列表为空。</dd></div>
        </dl>
        <h3>从项目根目录启动桌面窗口</h3>
        <p className="runtime-help-note">首次准备时获取脚本；已有可用 script/ 可跳过第一行。需要 Python、Git、Node.js、Go 和匹配的引擎资源。</p>
        <pre tabIndex={0}><code>{commands}</code></pre>
        <p className="runtime-help-note">资源检查：<code>python tools/check_resources.py --root .</code></p>
        <div className="runtime-help-actions"><button type="button" onClick={copy}>{copied?'已复制':'复制启动命令'}</button><a href="https://github.com/sjm1327605995/goygopro/blob/main/docs/setup-and-troubleshooting.md" target="_blank" rel="noreferrer">资源与排查文档 ↗</a><Dialog.Close>关闭</Dialog.Close></div>
        <p role="status" className="runtime-help-note">{copyFailed?'复制不可用，请在上方命令区手动选择复制。':copied?'已复制启动命令。':''}</p>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
