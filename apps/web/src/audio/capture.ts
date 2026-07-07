// Push-to-talk mic capture: getUserMedia -> AudioContext -> AudioWorklet
// (pcm-worklet.js just forwards raw Float32 frames) -> downsample to 16kHz
// mono Int16 chunks for the ASR bridge.

const TARGET_SAMPLE_RATE = 16000;

export class MicCapture {
  private stream: MediaStream | null = null;
  private ctx: AudioContext | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private node: AudioWorkletNode | null = null;

  async start(onPcm: (pcm: Int16Array) => void): Promise<void> {
    this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });

    const ctx = new AudioContext();
    this.ctx = ctx;
    await ctx.audioWorklet.addModule("/pcm-worklet.js");

    const node = new AudioWorkletNode(ctx, "pcm-worklet");
    this.node = node;
    node.port.onmessage = (ev: MessageEvent<Float32Array>) => {
      const pcm = downsampleToInt16(ev.data, ctx.sampleRate, TARGET_SAMPLE_RATE);
      if (pcm.length > 0) onPcm(pcm);
    };

    const source = ctx.createMediaStreamSource(this.stream);
    this.source = source;
    source.connect(node);
  }

  stop(): void {
    this.stream?.getTracks().forEach((track) => track.stop());
    this.source?.disconnect();
    this.node?.disconnect();
    void this.ctx?.close();
    this.stream = null;
    this.source = null;
    this.node = null;
    this.ctx = null;
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
