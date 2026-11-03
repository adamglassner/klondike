"""Packs the exported art/audio into the Go game's embedded assets:
cards -> lossless WebP, audio -> IMA ADPCM (.ima, 4:1)."""
import sys, os, json, glob, struct, shutil
import numpy as np
from PIL import Image
SRC, DST = sys.argv[1], sys.argv[2]
os.makedirs(f'{DST}/cards', exist_ok=True); os.makedirs(f'{DST}/audio', exist_ok=True)
for f in glob.glob(f'{SRC}/cards/*.png'):
    Image.open(f).save(f'{DST}/cards/' + os.path.basename(f)[:-4] + '.webp', 'WEBP', lossless=True, method=6)
shutil.copy(f'{SRC}/cards/cards.json', f'{DST}/cards/cards.json')
shutil.copy(f'{SRC}/table.jpg', f'{DST}/table.jpg')

STEP = [7,8,9,10,11,12,13,14,16,17,19,21,23,25,28,31,34,37,41,45,50,55,60,66,73,80,88,97,107,118,130,143,157,173,190,209,230,253,279,307,337,371,408,449,494,544,598,658,724,796,876,963,1060,1166,1282,1411,1552,1707,1878,2066,2272,2499,2749,3024,3327,3660,4026,4428,4871,5358,5894,6484,7132,7845,8630,9493,10442,11487,12635,13899,15289,16818,18500,20350,22385,24623,27086,29794,32767]
IDX = [-1,-1,-1,-1,2,4,6,8,-1,-1,-1,-1,2,4,6,8]
BLOCK = 1024  # frames per block; each block restarts the predictor for both channels

def encode_channel(x):
    """IMA ADPCM; returns (nibbles, pred0, index0) per block."""
    out = []
    pred, idx = int(x[0]), 0
    for b in range(0, len(x), BLOCK):
        seg = x[b:b + BLOCK]
        hdr = (pred, idx); nib = bytearray(len(seg))
        for i, s in enumerate(seg):
            step = STEP[idx]; diff = int(s) - pred; code = 0
            if diff < 0: code = 8; diff = -diff
            delta = step >> 3
            if diff >= step: code |= 4; diff -= step; delta += step
            if diff >= step >> 1: code |= 2; diff -= step >> 1; delta += step >> 1
            if diff >= step >> 2: code |= 1; delta += step >> 2
            pred = pred - delta if code & 8 else pred + delta
            pred = -32768 if pred < -32768 else 32767 if pred > 32767 else pred
            idx += IDX[code]; idx = 0 if idx < 0 else 88 if idx > 88 else idx
            nib[i] = code
        out.append((hdr, nib))
    return out

meta = json.load(open(f'{SRC}/audio/audio.json'))
for f in sorted(glob.glob(f'{SRC}/audio/*.pcm')):
    pcm = np.frombuffer(open(f, 'rb').read(), dtype='<i2').reshape(-1, 2)
    L, R = encode_channel(pcm[:, 0]), encode_channel(pcm[:, 1])
    buf = bytearray(b'KIMA') + struct.pack('<IHI', meta['rate'], 2, len(pcm))
    for (hl, nl), (hr, nr) in zip(L, R):
        buf += struct.pack('<hBhB', hl[0], hl[1], hr[0], hr[1])
        buf += bytes(a | (b << 4) for a, b in zip(nl, nr))
    name = os.path.basename(f)[:-4]
    open(f'{DST}/audio/{name}.ima', 'wb').write(buf)
    print(name, len(pcm), '->', len(buf))
json.dump(meta, open(f'{DST}/audio/audio.json', 'w'))
