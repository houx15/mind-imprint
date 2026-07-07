// Push-to-talk mic capture: getUserMedia -> AudioContext -> AudioWorklet
// (pcm-worklet.js just forwards raw Float32 frames) -> downsample to 16kHz
// mono Int16 chunks for the ASR bridge.

const TARGET_SAMPLE_RATE = 16000;
// ~100ms of audio at 16kHz. PCM is batched to this size before being handed
// to the caller, instead of firing on every ~128-sample worklet quantum
// (which would otherwise mean a WS frame every couple of milliseconds).
const FRAME_SAMPLES = 1600;

export class MicCapture {
  private stream: MediaStream | null = null;
  private ctx: AudioContext | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private node: AudioWorkletNode | null = null;
  private cancelled = false;
  private pending: Int16Array[] = [];
  private pendingLength = 0;
  private onPcm: ((pcm: Int16Array) => void) | null = null;

  async start(onPcm: (pcm: Int16Array) => void): Promise<void> {
    this.cancelled = false;
    this.onPcm = onPcm;
    this.pending = [];
    this.pendingLength = 0;

    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    if (this.cancelled) {
      stream.getTracks().forEach((track) => track.stop());
      return;
    }
    this.stream = stream;

    const ctx = new AudioContext();
    this.ctx = ctx;
    await ctx.audioWorklet.addModule("/pcm-worklet.js");
    if (this.cancelled) {
      this.releaseAcquired();
      return;
    }

    const node = new AudioWorkletNode(ctx, "pcm-worklet");
    this.node = node;
    node.port.onmessage = (ev: MessageEvent<Float32Array>) => {
      const pcm = downsampleToInt16(ev.data, ctx.sampleRate, TARGET_SAMPLE_RATE);
      if (pcm.length > 0) this.bufferPcm(pcm);
    };

    const source = ctx.createMediaStreamSource(this.stream);
    this.source = source;
    source.connect(node);

    if (this.cancelled) {
      this.releaseAcquired();
    }
  }

  stop(): void {
    this.cancelled = true;
    this.flush();
    this.onPcm = null;
    this.releaseAcquired();
  }

  /** Release whatever mic/audio resources have been acquired so far. Safe to call repeatedly. */
  private releaseAcquired(): void {
    this.stream?.getTracks().forEach((track) => track.stop());
    this.source?.disconnect();
    this.node?.disconnect();
    void this.ctx?.close();
    this.stream = null;
    this.source = null;
    this.node = null;
    this.ctx = null;
  }

  private bufferPcm(pcm: Int16Array): void {
    this.pending.push(pcm);
    this.pendingLength += pcm.length;
    while (this.pendingLength >= FRAME_SAMPLES) {
      this.emitFrame(FRAME_SAMPLES);
    }
  }

  private emitFrame(size: number): void {
    const frame = new Int16Array(size);
    let filled = 0;
    while (filled < size && this.pending.length > 0) {
      const chunk = this.pending[0];
      if (!chunk) break;
      const needed = size - filled;
      if (chunk.length <= needed) {
        frame.set(chunk, filled);
        filled += chunk.length;
        this.pending.shift();
      } else {
        frame.set(chunk.subarray(0, needed), filled);
        this.pending[0] = chunk.subarray(needed);
        filled += needed;
      }
    }
    this.pendingLength -= filled;
    this.onPcm?.(frame);
  }

  /** Flush any remaining buffered samples (a partial, sub-100ms frame) on stop. */
  private flush(): void {
    if (this.pendingLength > 0) {
      this.emitFrame(this.pendingLength);
    }
  }
}

function downsampleToInt16(float32: Float32Array, inputRate: number, targetRate: number): Int16Array {
  if (targetRate >= inputRate) {
    return floatArrayToInt16(float32);
  }
  const ratio = inputRate / targetRate;
  const newLength = Math.round(float32.length / ratio);
  const result = new Int16Array(newLength);
  let offsetBuffer = 0;
  for (let i = 0; i < newLength; i++) {
    const nextOffsetBuffer = Math.round((i + 1) * ratio);
    let sum = 0;
    let count = 0;
    for (let j = offsetBuffer; j < nextOffsetBuffer && j < float32.length; j++) {
      sum += float32[j] ?? 0;
      count++;
    }
    result[i] = floatSampleToInt16(count > 0 ? sum / count : 0);
    offsetBuffer = nextOffsetBuffer;
  }
  return result;
}

function floatArrayToInt16(float32: Float32Array): Int16Array {
  const out = new Int16Array(float32.length);
  for (let i = 0; i < float32.length; i++) {
    out[i] = floatSampleToInt16(float32[i] ?? 0);
  }
  return out;
}

function floatSampleToInt16(sample: number): number {
  const clamped = Math.max(-1, Math.min(1, sample));
  return Math.round(clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff);
}
