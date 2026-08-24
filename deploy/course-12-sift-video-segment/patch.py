#!/usr/bin/env python3
"""Patch course-12's sift-check.html so a segment ending never reads as a freeze.

THE BUG (reported as "slice 4, step 2 - 停, the video pauses at 0:17")

Each rail step plays only a SLICE of a longer screen recording: SEG.stop is
v1 0:00-0:17.4 of a 42.1s file, SEG.find is 17.4-42.0 of that SAME file. A
`timeupdate` clamp pauses at `seg.end` and rewinds to `end - 0.05`.

That is the authored cut and it is correct. What was wrong is that watch mode
ALSO handed the learner the browser's NATIVE <video> controls, which advertise
the whole 42-second file: a scrubber sitting at 0:17 / 0:42 and a play button.
Both are lies. Pressing play resumes for one timeupdate tick and the clamp
pauses it again; dragging the scrubber anywhere past the segment end snaps
straight back. Verified live on prod: after the clamp, play() and a seek to
0:30 both leave `currentTime == 17.35, paused == true`.

So the learner sees a video that is two-fifths through, frozen, with a play
button that does nothing. It reads as broken.

THE FIX — stop lying about the timeline, and make the ending legible:

  1. Never attach native controls. They expose a timeline the code refuses to
     honour. The interaction's own chips become the only transport.
  2. Add a 播放/暂停 chip so pausing mid-segment is still possible (that was
     the only thing the native controls were genuinely good for), and let a
     click on the video do the same in watch mode.
  3. When the clamp fires, say so: the hint becomes "本段到这里就结束了…",
     and the chip becomes 重看本段. A deliberate ending, not a freeze.
  4. A slim progress bar inside the video frame, scaled to the SEGMENT rather
     than the file, so "how much of this clip is left" is visible at all.

Only the interaction asset changes. The CourseDefinition is untouched, so
there is no definition-hash change and no session reset.

    python3 patch.py            # writes patched/course-12__sift-check.html
"""
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "original", "sift-check.html")
OUT_DIR = os.path.join(HERE, "patched")
OUT = os.path.join(OUT_DIR, "course-12__sift-check.html")

# (label, needle, replacement) — every needle must appear EXACTLY once.
EDITS = [
    (
        "css: segment progress bar",
        ".vzinner video{width:100%;height:100%;display:block;background:#0d1219;object-fit:contain}",
        ".vzinner video{width:100%;height:100%;display:block;background:#0d1219;object-fit:contain}\n"
        ".vprog{position:absolute;left:0;bottom:0;height:3px;width:0;background:#2f6fd0;"
        "pointer-events:none;transition:width .2s linear}",
    ),
    (
        "markup: progress bar inside the video frame",
        '<div class="videobox" id="vbox"><video id="vid" playsinline muted preload="auto"></video></div>',
        '<div class="videobox" id="vbox"><video id="vid" playsinline muted preload="auto"></video>'
        '<i class="vprog" id="vprogbar"></i></div>',
    ),
    (
        "markup: 播放/暂停 chip",
        '<button class="chip" id="replayBtn">重播</button>',
        '<button class="chip" id="playBtn">暂停</button>\n'
        '        <button class="chip" id="replayBtn">重播</button>',
    ),
    (
        "js: segEnded state + chip sync + restart clears it",
        """function restart(){
  if(!seg) return;
  try { vid.currentTime = seg.start; } catch(e){}
  var p = vid.play();
  if(p && p.catch) p.catch(function(){});
}""",
        """/* True once the clamp below has parked the video at this segment's end.
   The segment is a slice of a longer file, so "resume" there is meaningless —
   the only sensible move is to replay the segment. */
var segEnded = false;
function playVid(){
  var p = vid.play();
  if(p && p.catch) p.catch(function(){});
}
function syncPlayChip(){
  var b = el('playBtn');
  if(!b) return;
  b.textContent = segEnded ? '重看本段' : (vid.paused ? '播放' : '暂停');
}
function setProgress(){
  var bar = el('vprogbar');
  if(!bar) return;
  if(!seg){ bar.style.width = '0%'; return; }
  var span = Math.max(0.001, seg.end - seg.start);
  var pct = Math.max(0, Math.min(100, (vid.currentTime - seg.start) / span * 100));
  bar.style.width = pct.toFixed(1) + '%';
}
function restart(){
  if(!seg) return;
  segEnded = false;
  el('vhint').textContent = seg.hint;
  try { vid.currentTime = seg.start; } catch(e){}
  playVid();
  setProgress();
  syncPlayChip();
}""",
    ),
    (
        "js: clamp announces the ending instead of just freezing",
        """  if(vid.currentTime >= seg.end){
    vid.pause();
    try { vid.currentTime = seg.end - 0.05; } catch(e){}
    if(mode === 'watch') el('goBtn').textContent = '看完了，动手做 →';
  }
});""",
        """  setProgress();
  if(vid.currentTime >= seg.end){
    vid.pause();
    try { vid.currentTime = seg.end - 0.05; } catch(e){}
    if(!segEnded){
      segEnded = true;
      el('vprogbar').style.width = '100%';
      el('vhint').textContent = '本段到这里结束——这一步只截取了录屏的其中一节。想再看一遍，点「重看本段」。';
      syncPlayChip();
    }
    if(mode === 'watch') el('goBtn').textContent = '看完了，动手做 →';
  }
});
vid.addEventListener('play', syncPlayChip);
vid.addEventListener('pause', syncPlayChip);
el('playBtn').onclick = function(){
  if(segEnded){ restart(); return; }
  if(vid.paused) playVid(); else vid.pause();
  syncPlayChip();
};""",
    ),
    (
        "js: clicking the frame works in watch mode too",
        """el('vbox').onclick = function(){
  if(mode !== 'do') return;
  if(vid.paused) restart(); else vid.pause();
};""",
        """el('vbox').onclick = function(){
  if(mode === 'do'){ if(vid.paused) restart(); else vid.pause(); return; }
  if(segEnded) restart();
  else if(vid.paused) playVid();
  else vid.pause();
  syncPlayChip();
};""",
    ),
    (
        "js: zoom overlay play chip must replay a finished segment",
        """el('vzPlay').onclick = function(){
  if(vid.paused){ var p = vid.play(); if(p && p.catch) p.catch(function(){}); this.textContent = '暂停'; }
  else { vid.pause(); this.textContent = '播放'; }
};""",
        """el('vzPlay').onclick = function(){
  if(segEnded){ restart(); this.textContent = '暂停'; return; }
  if(vid.paused){ playVid(); this.textContent = '暂停'; }
  else { vid.pause(); this.textContent = '播放'; }
};""",
    ),
    (
        "js: zoom overlay label reflects a finished segment",
        "  el('vzPlay').textContent = vid.paused ? '播放' : '暂停';",
        "  el('vzPlay').textContent = segEnded ? '重看本段' : (vid.paused ? '播放' : '暂停');",
    ),
    (
        "js: never attach the native controls",
        """function syncControls(){
  if(mode === 'watch' && seg) vid.setAttribute('controls','controls');
  else vid.removeAttribute('controls');
}""",
        """function syncControls(){
  /* Never the native <video> controls. Each step plays only SEG.start→SEG.end
     of a longer recording and the timeupdate clamp refuses to honour the
     native timeline, so the scrubber would advertise a 42s video whose play
     button does nothing once the segment ends. The chips are the transport. */
  vid.removeAttribute('controls');
  syncPlayChip();
}""",
    ),
]


def main() -> int:
    if not os.path.exists(SRC):
        sys.exit(f"missing source: {SRC}")
    html = open(SRC, encoding="utf-8").read()
    for label, needle, repl in EDITS:
        n = html.count(needle)
        if n != 1:
            sys.exit(f"REFUSING: '{label}' matched {n} times (expected exactly 1)")
        html = html.replace(needle, repl)
        print(f"  ok  {label}")
    os.makedirs(OUT_DIR, exist_ok=True)
    open(OUT, "w", encoding="utf-8").write(html)
    print(f"\nwrote {OUT} ({os.path.getsize(OUT)}B, source {os.path.getsize(SRC)}B)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
