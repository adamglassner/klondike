"""Exports Klondike's art and audio from the original web version (web/index.html)
so the Go version uses exactly the same graphics, music and sounds.
Card text is not baked in: every fillText call is recorded instead, so the
game can draw it with Windows' own Georgia at run time."""
import asyncio, base64, json, os, struct, sys
from playwright.async_api import async_playwright
SRC = sys.argv[1]; OUT = sys.argv[2]
os.makedirs(f'{OUT}/cards', exist_ok=True); os.makedirs(f'{OUT}/audio', exist_ok=True)
CW0 = 280
JS_ART = r'''
async (CW0) => {
  const ops = [];
  const orig = CanvasRenderingContext2D.prototype.fillText;
  CanvasRenderingContext2D.prototype.fillText = function (text, x, y) {
    const m = this.getTransform();
    ops.push({ text, x, y, font: this.font, color: this.fillStyle, align: this.textAlign, baseline: this.textBaseline,
               m: [m.a, m.b, m.c, m.d, m.e, m.f] });
  };
  dpr = 1; cw = CW0; ch = Math.round(CW0 * 1.4); pad = Math.ceil(cw * .08);
  const out = { cw, ch, pad, cards: {} };
  const take = (name, canvas) => { out.cards[name] = { png: canvas.toDataURL('image/png'), text: ops.splice(0) }; };
  for (let id = 0; id < 52; id++) take('c' + id, renderFace(id));
  take('back', renderBack());
  // empty-pile slots, drawn exactly like the game does
  const src = [...document.scripts].map(s => s.textContent).join('\n');
  const a = src.indexOf('function slot(x,y,label){'), b = src.indexOf('\nfunction frame(');
  for (const label of ['', 'A', 'R']) {
    const c = document.createElement('canvas'); c.width = cw + 2 * pad; c.height = ch + 2 * pad;
    const x = c.getContext('2d'); x.translate(pad, pad);
    const slotFn = new Function('ctx', src.slice(a, b) + '\nreturn slot;')(x);
    slotFn(0, 0, label);
    take('slot' + (label || '_'), c);
  }
  // the shadow under a card being dragged: drawCard(..., shadow=true)
  { const m = 48, c = document.createElement('canvas'); c.width = cw + 2 * pad + 2 * m; c.height = ch + 2 * pad + 2 * m;
    const x = c.getContext('2d'); x.shadowColor = 'rgba(0,0,0,.45)'; x.shadowBlur = 22; x.shadowOffsetY = 12; x.shadowOffsetX = 10000;
    x.drawImage(renderBack(), m - 10000, m); out.liftMargin = m; take('lift', c); }
  // the plank table
  W = 2560; H = 1440; const t = renderTable(); out.table = t.toDataURL('image/jpeg', .92);
  CanvasRenderingContext2D.prototype.fillText = orig;
  return out;
}'''
JS_AUDIO = r'''
async ([what, rate]) => {
  const src = [...document.scripts].map(s => s.textContent).join('\n');
  let m = src.slice(src.indexOf('const Music=(()=>{'), src.indexOf('const Sfx={'));
  m = m.replace('const Music=', 'return ').replace('ac=new (window.AudioContext||window.webkitAudioContext)();', 'ac=window.__ACTX;')
       .replace("sfx(fn){if(!ac||!settings.sfx||ac.state!=='running')return;fn(ac,sfxGain,noise);}",
                "sfx(fn){fn(ac,sfxGain,noise);}, __music(){ensure(); musicGain.gain.value=.9;}, __len(i){ensure(); return compiled[i].len*compiled[i].e8;}, __tune(i){ensure(); musicGain.gain.value=.9; const c=compiled[i]; for(const e of c.ev) play(e,.1+e.t*c.e8,c.e8);}, __name(i){ensure(); return compiled[i].name;}");
  let s = src.slice(src.indexOf('const Sfx={'), src.indexOf('/* ---------------------------------------------------------------\n   Boot'));
  s = s.replace('const Sfx=', 'return ');
  const make = ctx => { window.__ACTX = ctx; const M = new Function(m)(); const S = new Function('Music', s)(M); return { M, S }; };
  let dur;
  if (what.startsWith('tune')) { const i = +what.slice(4); const probe = make(new OfflineAudioContext(2, rate, rate)); dur = probe.M.__len(i) + 4.5; }
  else dur = { place: .4, flip: .3, riffle: .7, shuffle: 2.4, fanfare: 4.2 }[what.replace(/\d+$/, '')];
  const ctx = new OfflineAudioContext(2, Math.ceil(dur * rate), rate);
  const { M, S } = make(ctx);
  let name = '';
  if (what.startsWith('tune')) { M.__tune(+what.slice(4)); name = M.__name(+what.slice(4)); }
  else if (what.startsWith('fanfare')) { M.__music(); M.fanfare(); }
  else { M.__music(); S[what.replace(/\d+$/, '')](); }
  const buf = await ctx.startRendering();
  const L = buf.getChannelData(0), R = buf.getChannelData(1), n = L.length;
  // trim trailing silence
  let end = n; while (end > 0 && Math.abs(L[end - 1]) < 2e-4 && Math.abs(R[end - 1]) < 2e-4) end--;
  const pcm = new Int16Array(end * 2);
  for (let i = 0; i < end; i++) { pcm[2 * i] = Math.max(-1, Math.min(1, L[i])) * 32767; pcm[2 * i + 1] = Math.max(-1, Math.min(1, R[i])) * 32767; }
  let bin = ''; const u8 = new Uint8Array(pcm.buffer); for (let i = 0; i < u8.length; i += 0x8000) bin += String.fromCharCode.apply(null, u8.subarray(i, i + 0x8000));
  return { name, frames: end, b64: btoa(bin) };
}'''
async def main():
    async with async_playwright() as p:
        b = await p.chromium.launch()
        pg = await b.new_page()
        await pg.goto('file://' + os.path.abspath(SRC)); await pg.wait_for_timeout(800)
        r = await pg.evaluate(JS_ART, CW0)
        meta = {'cw': r['cw'], 'ch': r['ch'], 'pad': r['pad'], 'liftMargin': r['liftMargin'], 'text': {}}
        for name, c in r['cards'].items():
            open(f'{OUT}/cards/{name}.png', 'wb').write(base64.b64decode(c['png'].split(',')[1]))
            meta['text'][name] = c['text']
        open(f'{OUT}/table.jpg', 'wb').write(base64.b64decode(r['table'].split(',')[1]))
        json.dump(meta, open(f'{OUT}/cards/cards.json', 'w'), separators=(',', ':'))
        print('art: %d images' % len(r['cards']))
        RATE = 32000
        jobs = [f'tune{i}' for i in range(4)] + [f'place{i}' for i in range(4)] + [f'flip{i}' for i in range(3)] + ['riffle0', 'riffle1', 'shuffle0', 'fanfare0']
        names = {}
        for j in jobs:
            a = await pg.evaluate(JS_AUDIO, [j, RATE])
            open(f'{OUT}/audio/{j}.pcm', 'wb').write(base64.b64decode(a['b64']))
            if a['name']: names[j] = a['name']
            print(j, a['name'], '%.1fs' % (a['frames'] / RATE))
        json.dump({'rate': RATE, 'names': names}, open(f'{OUT}/audio/audio.json', 'w'))
        await b.close()
asyncio.run(main())
