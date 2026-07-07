// Minimal AudioWorkletProcessor: forwards raw Float32 mic frames to the
// main thread. All resampling (ctx.sampleRate -> 16kHz) and Int16
// conversion happens in src/audio/capture.ts to keep this file trivial.
class PcmWorkletProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const input = inputs[0];
    const channel = input && input[0];
    if (channel && channel.length > 0) {
      this.port.postMessage(channel.slice());
    }
    return true;
  }
}

registerProcessor("pcm-worklet", PcmWorkletProcessor);
