import "./portfolioShare.css";

import { useEffect, useRef, useState } from "react";
export function PortfolioShareDialog({url,onClose}:{url:string;onClose:()=>void}) {
  const ref=useRef<HTMLDialogElement>(null);
  const [copied,setCopied]=useState(false);
  const [error,setError]=useState('');
  useEffect(()=>{const el=ref.current;el?.showModal();return()=>el?.close();},[]);
  const text=`快来看看我的个人作品集吧：${url}`;
  return <dialog ref={ref} className="portfolio-share-dialog" onCancel={onClose} onClose={onClose} aria-labelledby="portfolio-share-title">
    <button type="button" className="portfolio-share-close" aria-label="关闭分享窗口" onClick={onClose}>×</button>
    <p>发布成功</p><h2 id="portfolio-share-title">分享我的个人作品集</h2>
    <p>复制分享文案，把主页发给朋友。</p>
    <textarea readOnly aria-label="分享文案" value={text} rows={3}/>
    <a href={url} target="_blank" rel="noopener noreferrer">查看个人主页</a>
    <button type="button" className="portfolio-share-copy" onClick={()=>{void navigator.clipboard.writeText(text).then(()=>{setCopied(true);setError('');}).catch(()=>setError('复制失败，请选中上方文案手动复制'));}}>{copied?'已复制':'复制分享文案'}</button>
    {error&&<p role="alert">{error}</p>}
  </dialog>;
}
